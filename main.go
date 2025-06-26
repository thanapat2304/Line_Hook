package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
	"github.com/line/line-bot-sdk-go/linebot"
)

var (
	db  *sql.DB
	bot *linebot.Client
	ctx = context.Background()
)

type Config struct {
	ChannelSecret string `json:"channelSecret"`
	ChannelToken  string `json:"channelToken"`
}

func loadConfig() (*Config, error) {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}

// connectDB เชื่อมต่อ PostgreSQL ด้วย database/sql
func connectDB() {
	var err error
	dsn := "host=150.95.30.116 port=5432 user=postgres password=BCpwd#123! dbname=postgres sslmode=disable"
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Cannot open DB:", err)
	}
	// ทดสอบการเชื่อมต่อ DB
	if err = db.PingContext(ctx); err != nil {
		log.Fatal("Cannot connect to DB:", err)
	}
}

// getGroupID ดึง groupId จาก customer_code
func getGroupID(customerCode string) (string, error) {
	var groupID string
	err := db.QueryRowContext(ctx, "SELECT group_id FROM customer_groups WHERE customer_code=$1", customerCode).Scan(&groupID)
	return groupID, err
}

// pushMessage ส่งข้อความเข้า LINE group
func pushMessage(groupID, message string) error {
	_, err := bot.PushMessage(groupID, linebot.NewTextMessage(message)).Do()
	return err
}

func main() {
	fmt.Println("=== STARTING SERVER ===")
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal("Cannot load config:", err)
	}
	channelSecret := cfg.ChannelSecret
	channelToken := cfg.ChannelToken

	if channelSecret == "" || channelToken == "" {
		log.Fatal("missing channelSecret or channelToken in config.json")
	}

	bot, err = linebot.New(channelSecret, channelToken)
	if err != nil {
		log.Fatal(err)
	}

	connectDB()
	defer db.Close()

	r := gin.Default()

	// Route สำหรับรับ Webhook จาก Grafana
	r.POST("/grafana-alert", func(c *gin.Context) {
		fmt.Println("==== GRAFANA ALERT HANDLER START ====")
		body, _ := ioutil.ReadAll(c.Request.Body)
		fmt.Printf("RAW BODY: %s\n", string(body))               // log raw body
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body)) // reset body for BindJSON

		var alertData map[string]interface{}
		if err := c.BindJSON(&alertData); err != nil {
			fmt.Printf("BIND ERROR: %v\n", err) // log bind error
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		fmt.Printf("PARSED: %+v\n", alertData) // log parsed data

		// ปรับให้รองรับโครงสร้าง JSON ของ Grafana (alerts[0].labels, alerts[0].annotations)
		alerts, ok := alertData["alerts"].([]interface{})
		if !ok || len(alerts) == 0 {
			fmt.Println("==== ALERTS NOT FOUND OR EMPTY ====")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing alerts"})
			return
		}
		firstAlert, ok := alerts[0].(map[string]interface{})
		if !ok {
			fmt.Println("==== FIRST ALERT FORMAT ERROR ====")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid alert format"})
			return
		}
		labels, ok := firstAlert["labels"].(map[string]interface{})
		if !ok {
			fmt.Println("==== LABELS NOT FOUND ====")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing labels"})
			return
		}
		customerCode, ok := labels["customer"].(string)
		if !ok {
			fmt.Println("==== CUSTOMER LABEL NOT FOUND ====")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing customer label"})
			return
		}

		// สร้างข้อความจากหลายแหล่งข้อมูล
		var message string

		annotations, _ := firstAlert["annotations"].(map[string]interface{})
		if summary, ok := annotations["summary"].(string); ok && summary != "" {
			message = fmt.Sprintf("[ALERT] %s", summary)
		} else {
			if values, ok := firstAlert["values"].(map[string]interface{}); ok {
				fmt.Printf("values: %+v\n", values)
				bValue := values["B"]
				fmt.Printf("values[\"B\"]: %+v, type: %T\n", bValue, bValue)
				switch v := bValue.(type) {
				case float64:
					message = fmt.Sprintf("ฉุกเฉินอุณหภูมิสูงกว่าค่าที่กำหนด %.1f องศา", v)
				case string:
					message = fmt.Sprintf("ฉุกเฉินอุณหภูมิสูงกว่าค่าที่กำหนด %s องศา", v)
				default:
					message = "[ALERT] Unknown value"
				}
			} else {
				message = "[ALERT] Unknown alert"
			}
		}
		fmt.Printf("Message after custom: %s\n", message)

		fmt.Printf("Customer Code: %s\n", customerCode)
		fmt.Printf("Final Message: %s\n", message)

		groupID, err := getGroupID(customerCode)
		if err != nil {
			fmt.Println("==== UNKNOWN CUSTOMER ====")
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown customer"})
			return
		}

		err = pushMessage(groupID, message)
		if err != nil {
			fmt.Println("==== FAILED TO SEND MESSAGE ====")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
			return
		}

		fmt.Println("==== MESSAGE SENT SUCCESSFULLY ====")
		c.JSON(http.StatusOK, gin.H{"status": "message sent"})
	})

	r.Run(":8071")
}
