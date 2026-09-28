package main

import (
	"machine-service/database"
	"machine-service/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	// เริ่มต้นฐานข้อมูล
	database.ConnectDatabase()

	// เริ่มต้น Background Worker (Auto-Finish เมื่อซักครบเวลา)
	handlers.StartAutoFinishWorker()

	r := gin.Default()

	// เปิดใช้งาน CORS เพื่อให้ Frontend หรือ Service อื่นเรียกใช้งานได้
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, ngrok-skip-browser-warning")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// เสิร์ฟหน้าเว็บ Live Monitor Dashboard
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})
	r.GET("/dashboard", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// Health Check Route
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service":     "Machine Service",
			"status":      "running",
			"database":    "machine_db",
			"responsible": "ธนากร ไชยรัตน์ 67114540244",
		})
	})

	// เครื่องหมายเส้นทาง API ของ Machine Service
	machines := r.Group("/machines")
	{
		machines.GET("", handlers.GetMachines)
		machines.GET("/:id", handlers.GetMachineByID)
		machines.POST("", handlers.CreateMachine)
		machines.PUT("/:id", handlers.UpdateMachine)
		machines.DELETE("/:id", handlers.DeleteMachine)
		machines.POST("/:id/start", handlers.StartMachine)
		machines.POST("/:id/finish", handlers.FinishMachine)
		machines.GET("/:id/sessions", handlers.GetMachineSessions) // ประวัติการใช้งานของเครื่อง
	}

	// เส้นทางตรวจสอบประวัติ Session ทั้งหมด (สำหรับ Notification Service หรือ Admin)
	r.GET("/sessions", handlers.GetAllSessions)

	// ตั้งค่า Notification Service Webhook URL แบบ Real-time
	r.POST("/config/notification-url", handlers.SetNotificationURL)
	r.GET("/config/notification-url", handlers.GetNotificationURL)
	r.GET("/notifications/external", handlers.ProxyExternalNotifications)

	// รันเซิร์ฟเวอร์ที่ Port 8082
	r.Run(":8082")
}
