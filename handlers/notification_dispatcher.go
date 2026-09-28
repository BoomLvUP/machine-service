package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// NotificationServiceURL เก็บ URL ปลายทางของ Notification Service (ศรายุทธ)
var (
	NotificationServiceURL = "https://napkin-drinking-debunk.ngrok-free.dev/notifications"
	urlMutex               sync.RWMutex
)

// cleanNotificationURL ล้าง URL ป้องกันการใส่ slash ซ้ำ เช่น //notifications
func cleanNotificationURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "https://", "HTTPS_PLACEHOLDER_")
	raw = strings.ReplaceAll(raw, "http://", "HTTP_PLACEHOLDER_")
	for strings.Contains(raw, "//") {
		raw = strings.ReplaceAll(raw, "//", "/")
	}
	raw = strings.ReplaceAll(raw, "HTTPS_PLACEHOLDER_", "https://")
	raw = strings.ReplaceAll(raw, "HTTP_PLACEHOLDER_", "http://")
	raw = strings.TrimRight(raw, "/")
	if !strings.HasSuffix(raw, "/notifications") {
		raw = raw + "/notifications"
	}
	return raw
}

// ProxyExternalNotifications ดึงข้อมูลการแจ้งเตือนจาก Notification Service ของศรายุทธมาแสดงผล
func ProxyExternalNotifications(c *gin.Context) {
	urlMutex.RLock()
	baseURL := NotificationServiceURL
	urlMutex.RUnlock()

	baseURL = cleanNotificationURL(baseURL)

	targetURL := baseURL
	if u := c.Query("user_id"); u != "" {
		targetURL += "?user_id=" + u
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}
	req.Header.Set("ngrok-skip-browser-warning", "true")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Cannot reach Notification Service", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from Notification Service"})
		return
	}

	c.JSON(resp.StatusCode, data)
}

// SetNotificationURL อัปเดต URL ของ Notification Service ขณะรันระบบ
func SetNotificationURL(c *gin.Context) {
	var input struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'url' is required"})
		return
	}

	urlMutex.Lock()
	NotificationServiceURL = input.URL
	urlMutex.Unlock()

	log.Printf("🔗 [Notification Config] อัปเดต Notification Service URL เป็น: %s\n", input.URL)
	c.JSON(http.StatusOK, gin.H{
		"message": "Notification Service URL updated successfully",
		"url":     input.URL,
	})
}

// GetNotificationURL ดู URL ปัจจุบัน
func GetNotificationURL(c *gin.Context) {
	urlMutex.RLock()
	currentURL := NotificationServiceURL
	urlMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"notification_service_url": currentURL,
	})
}

// NotificationPayload โครงสร้างข้อมูลที่ส่งไปให้ตาราง notifications ของ Notification Service
// ตาม Schema ในเอกสาร: user_id, machine_id, message
type NotificationPayload struct {
	UserID    uint   `json:"user_id"`
	MachineID uint   `json:"machine_id"`
	Message   string `json:"message"`
	Status    string `json:"status,omitempty"` // STARTED, FINISHED, STOPPED
	CreatedAt string `json:"created_at"`
}

// SendNotificationRealtime ส่งข้อมูลแจ้งเตือนไปยัง Notification Service แบบ Real-time ทันที (Non-blocking)
func SendNotificationRealtime(userID uint, machineID uint, message string, status string) {
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	// 1. บันทึก Log ฝั่ง Machine Service เสมอ เพื่อให้เห็นการทำงานแบบ Real-time
	log.Printf("📢 [Real-time Notification] Machine #%d | User #%d | Status: %s | Message: %s\n", machineID, userID, status, message)

	urlMutex.RLock()
	targetURL := NotificationServiceURL
	urlMutex.RUnlock()

	if targetURL == "" {
		// ถ้ายังไม่ได้ตั้ง URL ปลายทาง ให้จบการทำงานตรงนี้ (Log ไว้อย่างเดียว)
		return
	}

	// 2. ส่ง HTTP POST แบบ Asynchronous ใน Goroutine (ไม่ทำให้ Machine Service ช้าหรือสะดุด)
	go func(url string, uID uint, mID uint, msg string, st string, dt string) {
		client := &http.Client{Timeout: 5 * time.Second}
		payload := NotificationPayload{
			UserID:    uID,
			MachineID: mID,
			Message:   msg,
			Status:    st,
			CreatedAt: dt,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
		if err != nil {
			log.Printf("⚠️ [Webhook Error] สร้าง Request ไม่สำเร็จ: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("ngrok-skip-browser-warning", "true")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("⚠️ [Webhook Error] ไม่สามารถยิงไปที่ Notification Service (%s): %v\n", url, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("✅ [Webhook Sent] ส่งแจ้งเตือน Real-time ไปยัง Notification Service สำเร็จ! (HTTP %d)\n", resp.StatusCode)
		} else {
			log.Printf("⚠️ [Webhook Sent] Notification Service ตอบกลับด้วย HTTP %d\n", resp.StatusCode)
		}
	}(targetURL, userID, machineID, message, status, nowStr)
}
