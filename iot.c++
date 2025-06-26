#include <OneWire.h>
#include <DallasTemperature.h>
#include <ESP8266WiFi.h>
#include <WiFiClient.h>
#include <ArduinoJson.h>

// กำหนดค่าคงที่
#define ONE_WIRE_BUS 2  // ขา GPIO2 (D4)
const char* ssid = "IOT-AEP_WIFI";
const char* password = "aepthailand1";

// ตั้งค่า Server
const char* serverHost = "150.95.30.116"; 
const int serverPort = 8070;

// เพิ่มประกาศสำหรับ LINE ALERT
const char* lineHost = "150.95.30.116";
const int linePort = 8071;
const char* customerCode = "customer_1";

// ตัวแปรระบบ
OneWire oneWire(ONE_WIRE_BUS);
DallasTemperature sensors(&oneWire);
WiFiClient client;
unsigned long lastSendTime = 0;
const unsigned long sendInterval = 60000; // ส่งข้อมูลทุก 1 นาที

// เพิ่มตัวแปรสำหรับ LINE ALERT
unsigned long lastLineAlertTime = 0;
const unsigned long lineAlertInterval = 600000; // 10 นาที (600,000 ms)
bool isTempAbnormal = false;
bool sensorErrorNotified = false;

void setup() {
  Serial.begin(115200);
  Serial.println("\nระบบตรวจสอบอุณหภูมิ DS18B20");
  
  // เริ่มเซ็นเซอร์
  sensors.begin();
  sensors.setResolution(12);
  
  // เชื่อมต่อ WiFi
  connectToWiFi();
}

void loop() {
  // ตรวจสอบ WiFi
  if (WiFi.status() != WL_CONNECTED) {
    connectToWiFi();
  }
  
  // อ่านและแสดงอุณหภูมิ
  sensors.requestTemperatures();
  float tempC = sensors.getTempCByIndex(0);
  
  if (tempC != DEVICE_DISCONNECTED_C) {
    Serial.printf("อุณหภูมิ: %.1f °C\n", tempC);
    
    // ส่งข้อมูลไปยังเซิร์ฟเวอร์ตามช่วงเวลา (1 นาที)
    if (millis() - lastSendTime >= sendInterval) {
      sendToDatabase(tempC);      // ส่งไปฐานข้อมูล (port 8070)
      lastSendTime = millis();
    }

    // เช็คช่วงอุณหภูมิผิดปกติ (นอก 20-25)
    bool tempAbnormalNow = (tempC < 20.0 || tempC > 25.0);
    unsigned long now = millis();
    if (tempAbnormalNow) {
      if (!isTempAbnormal || (now - lastLineAlertTime >= lineAlertInterval)) {
        sendToLineAlert(tempC); // ส่งแจ้งเตือน
        lastLineAlertTime = now;
        isTempAbnormal = true;
      }
      // ถ้ายังผิดปกติแต่ยังไม่ครบ 10 นาที จะไม่ส่งซ้ำ
    } else {
      // ถ้ากลับมาปกติ รีเซ็ตสถานะ
      isTempAbnormal = false;
    }
    // ถ้าเซนเซอร์กลับมาทำงาน รีเซ็ตสถานะ sensorErrorNotified
    sensorErrorNotified = false;
  } else {
    Serial.println("เกิดข้อผิดพลาดในการอ่านเซ็นเซอร์");
    // ส่ง 999 ไปฐานข้อมูลทุก 1 นาที
    if (millis() - lastSendTime >= sendInterval) {
      sendToDatabase(999);
      lastSendTime = millis();
    }
    // แจ้งเตือนไป LINE ว่าเซนเซอร์พัง (ครั้งเดียวจนกว่าจะกลับมาปกติ)
    if (!sensorErrorNotified) {
      sendSensorErrorToLine();
      sensorErrorNotified = true;
    }
  }
  
  delay(2000); // หน่วงเวลา 2 วินาที
}

void connectToWiFi() {
  Serial.println("กำลังเชื่อมต่อ WiFi...");
  WiFi.begin(ssid, password);
  
  int attempts = 0;
  while (WiFi.status() != WL_CONNECTED && attempts < 20) {
    delay(500);
    Serial.print(".");
    attempts++;
  }
  
  if (WiFi.status() == WL_CONNECTED) {
    Serial.println("\nเชื่อมต่อ WiFi สำเร็จ!");
    Serial.print("IP Address: ");
    Serial.println(WiFi.localIP());
  } else {
    Serial.println("\nเชื่อมต่อ WiFi ล้มเหลว!");
  }
}

void sendToDatabase(float temperature) {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("ไม่สามารถส่งข้อมูลได้ - WiFi ยังไม่เชื่อมต่อ");
    return;
  }

  // สร้าง JSON data
  StaticJsonDocument<256> doc;
  doc["device"] = "DS18B20_Sensor_2";
  doc["value"] = temperature;
  doc["branch"] = "Site_2";
  doc["mac"] = WiFi.macAddress();
  doc["sn"] = ESP.getChipId();
  
  String jsonData;
  serializeJson(doc, jsonData);
  
  Serial.println("กำลังส่งข้อมูล: " + jsonData);
  
  if (client.connect(serverHost, serverPort)) {
    client.println("POST /submit HTTP/1.1");
    client.println("Host: " + String(serverHost));
    client.println("Content-Type: application/json");
    client.println("Connection: close");
    client.print("Content-Length: ");
    client.println(jsonData.length());
    client.println();
    client.println(jsonData);
    
    // รอการตอบกลับ
    unsigned long timeout = millis();
    while (client.connected() && millis() - timeout < 5000) {
      if (client.available()) {
        String line = client.readStringUntil('\r');
        Serial.println("การตอบกลับ: " + line);
      }
    }
    client.stop();
    Serial.println("ส่งข้อมูลสำเร็จ");
  } else {
    Serial.println("การเชื่อมต่อเซิร์ฟเวอร์ล้มเหลว");
  }
}

void sendToLineAlert(float temperature) {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("ไม่สามารถส่งแจ้งเตือนไป LINE ได้ - WiFi ยังไม่เชื่อมต่อ");
    return;
  }

  WiFiClient lineClient;
  // สร้าง JSON data
  StaticJsonDocument<128> doc;
  doc["customer_code"] = customerCode;
  doc["temp_value"] = temperature;
  // เพิ่มสถานะสูง/ต่ำกว่าเกณฑ์
  if (temperature > 25.0) {
    doc["status"] = "สูงกว่าเกณฑ์";
  } else if (temperature < 20.0) {
    doc["status"] = "ต่ำกว่าเกณฑ์";
  } else {
    doc["status"] = "ปกติ";
  }
  doc["device"] = "DS18B20_Sensor_2";
  String jsonData;
  serializeJson(doc, jsonData);

  Serial.println("กำลังส่งแจ้งเตือนไป LINE: " + jsonData);

  if (lineClient.connect(lineHost, linePort)) {
    lineClient.println("POST /iot-alert HTTP/1.1");
    lineClient.println("Host: " + String(lineHost));
    lineClient.println("Content-Type: application/json");
    lineClient.println("Connection: close");
    lineClient.print("Content-Length: ");
    lineClient.println(jsonData.length());
    lineClient.println();
    lineClient.println(jsonData);

    // รอการตอบกลับ
    unsigned long timeout = millis();
    while (lineClient.connected() && millis() - timeout < 5000) {
      if (lineClient.available()) {
        String line = lineClient.readStringUntil('\r');
        Serial.println("การตอบกลับจาก LINE: " + line);
      }
    }
    lineClient.stop();
    Serial.println("ส่งแจ้งเตือนไป LINE สำเร็จ");
  } else {
    Serial.println("การเชื่อมต่อเซิร์ฟเวอร์ LINE ล้มเหลว");
  }
}

// เพิ่มฟังก์ชันแจ้งเตือนเซนเซอร์พัง
void sendSensorErrorToLine() {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("ไม่สามารถส่งแจ้งเตือนไป LINE ได้ - WiFi ยังไม่เชื่อมต่อ");
    return;
  }
  WiFiClient lineClient;
  StaticJsonDocument<128> doc;
  doc["customer_code"] = customerCode;
  doc["temp_value"] = 999;
  doc["status"] = "เซนเซอร์พัง";
  doc["device"] = "DS18B20_Sensor_2";
  String jsonData;
  serializeJson(doc, jsonData);
  Serial.println("กำลังส่งแจ้งเตือนเซนเซอร์พังไป LINE: " + jsonData);
  if (lineClient.connect(lineHost, linePort)) {
    lineClient.println("POST /iot-alert HTTP/1.1");
    lineClient.println("Host: " + String(lineHost));
    lineClient.println("Content-Type: application/json");
    lineClient.println("Connection: close");
    lineClient.print("Content-Length: ");
    lineClient.println(jsonData.length());
    lineClient.println();
    lineClient.println(jsonData);
    unsigned long timeout = millis();
    while (lineClient.connected() && millis() - timeout < 5000) {
      if (lineClient.available()) {
        String line = lineClient.readStringUntil('\r');
        Serial.println("การตอบกลับจาก LINE: " + line);
      }
    }
    lineClient.stop();
    Serial.println("ส่งแจ้งเตือนเซนเซอร์พังไป LINE สำเร็จ");
  } else {
    Serial.println("การเชื่อมต่อเซิร์ฟเวอร์ LINE ล้มเหลว");
  }
}