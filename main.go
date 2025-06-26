package main

import (
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
	if err != nil {
		if apiErr, ok := err.(*linebot.APIError); ok {
			fmt.Printf("LINE API error: Code %d, Message: %s, Details: %+v\n", apiErr.Code, apiErr.Response.Message, apiErr.Response.Details)
		} else {
			fmt.Printf("LINE API error: %v\n", err)
		}
	}
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
	// r.POST("/grafana-alert", ...) // ปิด endpoint นี้ ไม่ต้องรับจาก Grafana อีกต่อไป

	r.POST("/iot-alert", func(c *gin.Context) {
		fmt.Println("==== IOT ALERT HANDLER START ====")
		var req struct {
			CustomerCode string  `json:"customer_code"`
			TempValue    float64 `json:"temp_value"`
		}
		if err := c.BindJSON(&req); err != nil {
			fmt.Printf("BIND ERROR: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		fmt.Printf("IOT ALERT: customer_code=%s, temp=%.2f\n", req.CustomerCode, req.TempValue)

		groupID, err := getGroupID(req.CustomerCode)
		if err != nil {
			fmt.Println("==== UNKNOWN CUSTOMER ====")
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown customer"})
			return
		}

		fmt.Printf("groupID ที่จะส่ง: %s\n", groupID)

		message := fmt.Sprintf("ฉุกเฉินอุณหภูมิสูงกว่าค่าที่กำหนด %.1f องศา", req.TempValue)
		err = pushMessage(groupID, message)
		if err != nil {
			fmt.Printf("pushMessage error: %v\n", err)
			fmt.Println("==== FAILED TO SEND MESSAGE ====")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
			return
		}
		fmt.Println("==== IOT MESSAGE SENT SUCCESSFULLY ====")
		c.JSON(http.StatusOK, gin.H{"status": "message sent"})
	})

	r.Run(":8071")
}
