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
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing alerts"})
			return
		}
		firstAlert, ok := alerts[0].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid alert format"})
			return
		}
		labels, ok := firstAlert["labels"].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing labels"})
			return
		}
		customerCode, ok := labels["customer"].(string)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing customer label"})
			return
		}
		annotations, _ := firstAlert["annotations"].(map[string]interface{})
		summary, _ := annotations["summary"].(string)
		message := fmt.Sprintf("[ALERT] %s", summary)

		groupID, err := getGroupID(customerCode)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown customer"})
			return
		}

		err = pushMessage(groupID, message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "message sent"})
	})

	r.Run(":8071")
}
