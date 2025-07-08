package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

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

func pushMessage(groupID, device, status string, tempValue float64) error {
	altText := "แจ้งเตือนสถานะอุปกรณ์ตรวจวัดอุณหภูมิ"

	// ปรับเวลาเป็นเวลาไทย (UTC+7)
	thaiLoc := time.FixedZone("Asia/Bangkok", 7*60*60)
	now := time.Now().In(thaiLoc).Format("2006-01-02 15:04:05")

	var (
		bodyContents  []linebot.FlexComponent
		headerText    string
		headerColor   string
		headerBgColor string
		warningDetail []linebot.FlexComponent
	)

	// สร้างข้อความตามสถานะ
	if status == "เซนเซอร์มีปัญหา" {
		headerText = "🚨 อุปกรณ์ขัดข้อง"
		headerColor = "#FFFFFF"
		headerBgColor = "#D32F2F"

		warningDetail = []linebot.FlexComponent{
			&linebot.BoxComponent{
				Type:   linebot.FlexComponentTypeBox,
				Layout: linebot.FlexBoxLayoutTypeHorizontal,
				Margin: "md",
				Contents: []linebot.FlexComponent{
					&linebot.TextComponent{
						Type: linebot.FlexComponentTypeText,
						Text: "🔧",
						Size: "lg",
						Flex: linebot.IntPtr(0),
					},
					&linebot.TextComponent{
						Type:   linebot.FlexComponentTypeText,
						Text:   fmt.Sprintf("อุปกรณ์: %s", device),
						Size:   "md",
						Color:  "#333333",
						Flex:   linebot.IntPtr(1),
						Margin: "sm",
					},
				},
			},
			&linebot.BoxComponent{
				Type:   linebot.FlexComponentTypeBox,
				Layout: linebot.FlexBoxLayoutTypeHorizontal,
				Margin: "sm",
				Contents: []linebot.FlexComponent{
					&linebot.TextComponent{
						Type: linebot.FlexComponentTypeText,
						Text: "⚠️",
						Size: "lg",
						Flex: linebot.IntPtr(0),
					},
					&linebot.TextComponent{
						Type:   linebot.FlexComponentTypeText,
						Text:   "เซนเซอร์ตรวจวัดอุณหภูมิมีปัญหา กรุณาตรวจสอบโดยด่วน",
						Wrap:   true,
						Size:   "sm",
						Color:  "#D32F2F",
						Flex:   linebot.IntPtr(1),
						Margin: "sm",
					},
				},
			},
		}
	} else {
		headerText = "🚨 แจ้งเตือนอุณหภูมิ"
		headerColor = "#FFFFFF"
		headerBgColor = "#1976D2"

		warningDetail = []linebot.FlexComponent{
			&linebot.BoxComponent{
				Type:   linebot.FlexComponentTypeBox,
				Layout: linebot.FlexBoxLayoutTypeHorizontal,
				Margin: "md",
				Contents: []linebot.FlexComponent{
					&linebot.TextComponent{
						Type: linebot.FlexComponentTypeText,
						Text: "📱",
						Size: "md",
						Flex: linebot.IntPtr(0),
					},
					&linebot.TextComponent{
						Type:   linebot.FlexComponentTypeText,
						Text:   fmt.Sprintf("อุปกรณ์: %s", device),
						Size:   "sm",
						Color:  "#333333",
						Flex:   linebot.IntPtr(1),
						Margin: "sm",
					},
				},
			},
			&linebot.BoxComponent{
				Type:   linebot.FlexComponentTypeBox,
				Layout: linebot.FlexBoxLayoutTypeHorizontal,
				Margin: "sm",
				Contents: []linebot.FlexComponent{
					&linebot.TextComponent{
						Type: linebot.FlexComponentTypeText,
						Text: "📊",
						Size: "md",
						Flex: linebot.IntPtr(0),
					},
					&linebot.TextComponent{
						Type:   linebot.FlexComponentTypeText,
						Text:   fmt.Sprintf("สถานะ: %s", status),
						Size:   "sm",
						Color:  "#FFA000",
						Flex:   linebot.IntPtr(1),
						Margin: "sm",
					},
				},
			},
			&linebot.BoxComponent{
				Type:   linebot.FlexComponentTypeBox,
				Layout: linebot.FlexBoxLayoutTypeHorizontal,
				Margin: "sm",
				Contents: []linebot.FlexComponent{
					&linebot.TextComponent{
						Type: linebot.FlexComponentTypeText,
						Text: "🌡️",
						Size: "md",
						Flex: linebot.IntPtr(0),
					},
					&linebot.TextComponent{
						Type:   linebot.FlexComponentTypeText,
						Text:   fmt.Sprintf("อุณหภูมิ: %.1f °C", tempValue),
						Size:   "sm",
						Color:  "#D84315",
						Flex:   linebot.IntPtr(1),
						Margin: "sm",
					},
				},
			},
		}
	}

	// สร้าง header ของ Flex Message
	headerContents := []linebot.FlexComponent{
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   headerText,
			Weight: "bold",
			Size:   "xl",
			Color:  headerColor,
			Align:  linebot.FlexComponentAlignTypeCenter,
		},
	}

	// สร้าง body ของ Flex Message
	bodyContents = []linebot.FlexComponent{
		&linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Spacing:  "md",
			Contents: warningDetail,
		},
	}

	// สร้าง footer ของ Flex Message
	footerContents := []linebot.FlexComponent{
		&linebot.SeparatorComponent{
			Type:   linebot.FlexComponentTypeSeparator,
			Margin: "md",
		},
		&linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeHorizontal,
			Contents: []linebot.FlexComponent{
				&linebot.TextComponent{
					Type: linebot.FlexComponentTypeText,
					Text: "🕒",
					Size: "sm",
					Flex: linebot.IntPtr(0),
				},
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   "แจ้งเตือนเมื่อ: " + now,
					Size:   "xs",
					Color:  "#888888",
					Flex:   linebot.IntPtr(1),
					Margin: "sm",
				},
			},
		},
	}

	// สร้าง Bubble message
	bubble := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Header: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: headerContents,
		},
		Body: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Spacing:  "md",
			Contents: bodyContents,
		},
		Footer: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: footerContents,
		},
		Styles: &linebot.BubbleStyle{
			Header: &linebot.BlockStyle{
				BackgroundColor: headerBgColor,
			},
			Body: &linebot.BlockStyle{
				BackgroundColor: "#FFFFFF",
			},
			Footer: &linebot.BlockStyle{
				BackgroundColor: "#F8F9FA",
			},
		},
	}

	_, err := bot.PushMessage(groupID, linebot.NewFlexMessage(altText, bubble)).Do()
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

	r.POST("/iot-alert", func(c *gin.Context) {
		fmt.Println("==== IOT ALERT HANDLER START ====")
		var req struct {
			CustomerCode string  `json:"customer_code"`
			TempValue    float64 `json:"temp_value"`
			Status       string  `json:"status"`
			Device       string  `json:"device"`
		}
		if err := c.BindJSON(&req); err != nil {
			fmt.Printf("BIND ERROR: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
		fmt.Printf("IOT ALERT: customer_code=%s, temp=%.2f, status=%s\n", req.CustomerCode, req.TempValue, req.Status)

		groupID, err := getGroupID(req.CustomerCode)
		if err != nil {
			fmt.Println("==== UNKNOWN CUSTOMER ====")
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown customer"})
			return
		}

		fmt.Printf("DEBUG: groupID=%s\n", groupID)
		err = pushMessage(groupID, req.Device, req.Status, req.TempValue)
		if err != nil {
			fmt.Printf("==== FAILED TO SEND MESSAGE: %v ====", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message", "detail": err.Error()})
			return
		}
		fmt.Println("==== IOT MESSAGE SENT SUCCESSFULLY ====")
		c.JSON(http.StatusOK, gin.H{"status": "message sent"})
	})

	r.Run(":8071")
}
