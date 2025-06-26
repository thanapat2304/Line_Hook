<<<<<<< HEAD
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
=======
package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type SensorData struct {
	Device string  `json:"device"`
	Value  float64 `json:"value"`
	Branch string  `json:"branch"`
	Mac    string  `json:"mac"`
	SN     int     `json:"sn"`
}

type SensorRecord struct {
	ID        int       `json:"id"`
	Device    string    `json:"device"`
	Value     float64   `json:"value"`
	Branch    string    `json:"branch"`
	Timestamp time.Time `json:"timestamp"`
	MacAdd    string    `json:"mac_add"`
	SerialNum int       `json:"serial_num"`
}

func main() {
	db, err := sql.Open("postgres", "host=150.95.30.116 port=5432 user=postgres password=BCpwd#123! dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// ทดสอบการเชื่อมต่อฐานข้อมูล
	err = db.Ping()
	if err != nil {
		log.Fatal("ไม่สามารถเชื่อมต่อฐานข้อมูลได้:", err)
	}
	log.Println("เชื่อมต่อฐานข้อมูลสำเร็จ")

	r := gin.Default()

	// เพิ่ม CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Endpoint สำหรับรับข้อมูลจาก IoT device
	r.POST("/submit", func(c *gin.Context) {
		var data SensorData
		if err := c.ShouldBindJSON(&data); err != nil {
			log.Printf("ข้อผิดพลาดในการแปลง JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON", "details": err.Error()})
			return
		}

		// ตรวจสอบข้อมูลที่จำเป็น
		if data.Device == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Device name is required"})
			return
		}

		// ตั้งค่าสาขาเริ่มต้นถ้าไม่ระบุ
		if data.Branch == "" {
			data.Branch = "ศรีราชา"
		}

		query := "INSERT INTO sensor_table (device, value, branch, mac_add, serial_num) VALUES ($1, $2, $3, $4, $5)"
		result, err := db.Exec(query, data.Device, data.Value, data.Branch, data.Mac, data.SN)
		if err != nil {
			log.Printf("ข้อผิดพลาดในการบันทึกฐานข้อมูล: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB insert failed", "details": err.Error()})
			return
		}

		id, _ := result.LastInsertId()
		log.Printf("บันทึกข้อมูลสำเร็จ - ID: %d, Device: %s, Value: %.1f°C, Branch: %s, MAC: %s, SN: %d", id, data.Device, data.Value, data.Branch, data.Mac, data.SN)

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Data saved successfully",
			"id":      id,
			"device":  data.Device,
			"value":   data.Value,
			"branch":  data.Branch,
			"mac":     data.Mac,
			"sn":      data.SN,
		})
	})

	// Endpoint สำหรับดึงข้อมูลจากฐานข้อมูล
	r.GET("/sensors", func(c *gin.Context) {
		limit := c.DefaultQuery("limit", "100")
		branch := c.Query("branch")

		var query string
		var args []interface{}

		if branch != "" {
			query = "SELECT id, device, value, branch, timestamp, mac_add, serial_num FROM sensor_table WHERE branch = $1 ORDER BY timestamp DESC LIMIT $2"
			args = []interface{}{branch, limit}
		} else {
			query = "SELECT id, device, value, branch, timestamp, mac_add, serial_num FROM sensor_table ORDER BY timestamp DESC LIMIT $1"
			args = []interface{}{limit}
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("ข้อผิดพลาดในการดึงข้อมูล: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch data", "details": err.Error()})
			return
		}
		defer rows.Close()

		var records []SensorRecord
		for rows.Next() {
			var record SensorRecord
			err := rows.Scan(&record.ID, &record.Device, &record.Value, &record.Branch, &record.Timestamp, &record.MacAdd, &record.SerialNum)
			if err != nil {
				log.Printf("ข้อผิดพลาดในการอ่านข้อมูล: %v", err)
				continue
			}
			records = append(records, record)
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  len(records),
			"branch": branch,
			"data":   records,
		})
	})

	// Endpoint สำหรับดึงข้อมูลล่าสุด
	r.GET("/sensors/latest", func(c *gin.Context) {
		branch := c.Query("branch")

		var query string
		var args []interface{}

		if branch != "" {
			query = "SELECT id, device, value, branch, timestamp, mac_add, serial_num FROM sensor_table WHERE branch = $1 ORDER BY timestamp DESC LIMIT 1"
			args = []interface{}{branch}
		} else {
			query = "SELECT id, device, value, branch, timestamp, mac_add, serial_num FROM sensor_table ORDER BY timestamp DESC LIMIT 1"
			args = []interface{}{}
		}

		var record SensorRecord
		var err error

		if len(args) > 0 {
			err = db.QueryRow(query, args...).Scan(&record.ID, &record.Device, &record.Value, &record.Branch, &record.Timestamp, &record.MacAdd, &record.SerialNum)
		} else {
			err = db.QueryRow(query).Scan(&record.ID, &record.Device, &record.Value, &record.Branch, &record.Timestamp, &record.MacAdd, &record.SerialNum)
		}

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "No data found"})
				return
			}
			log.Printf("ข้อผิดพลาดในการดึงข้อมูลล่าสุด: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch latest data", "details": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"branch": branch,
			"data":   record,
		})
	})

	// Endpoint สำหรับดูสถานะระบบ
	r.GET("/status", func(c *gin.Context) {
		err := db.Ping()
		dbStatus := "connected"
		if err != nil {
			dbStatus = "disconnected"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "running",
			"timestamp": time.Now(),
			"database":  dbStatus,
			"version":   "1.0.0",
		})
	})

	// Endpoint สำหรับดูรายการสาขาทั้งหมด
	r.GET("/branches", func(c *gin.Context) {
		query := "SELECT DISTINCT branch FROM sensor_table ORDER BY branch"
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("ข้อผิดพลาดในการดึงรายการสาขา: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch branches", "details": err.Error()})
			return
		}
		defer rows.Close()

		var branches []string
		for rows.Next() {
			var branch string
			err := rows.Scan(&branch)
			if err != nil {
				log.Printf("ข้อผิดพลาดในการอ่านสาขา: %v", err)
				continue
			}
			branches = append(branches, branch)
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "success",
			"count":    len(branches),
			"branches": branches,
		})
	})

	log.Println("เริ่มต้นเซิร์ฟเวอร์ที่ port 8070...")
	r.Run(":8070")
}
>>>>>>> b3c1019590f2da421bd51a469bdb96d55a94acc4
