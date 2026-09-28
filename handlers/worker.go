package handlers

import (
	"fmt"
	"log"
	"machine-service/database"
	"machine-service/models"
	"time"
)

// ResetDelaySeconds ระยะเวลาที่เครื่องจะค้างสถานะ FINISHED ไว้เพื่อให้ Notification Service ดึงข้อมูล (เช่น 30 วินาที)
// หลังจากนั้นระบบจะรีเซ็ตสถานะเครื่องกลับเป็น AVAILABLE อัตโนมัติ โดยที่ผู้ใช้ไม่ต้องกดเอง
const ResetDelaySeconds = 30

// StartAutoFinishWorker เริ่มต้น Background Goroutine
// 1. เช็คเครื่องที่ซักครบเวลา -> เปลี่ยนเป็น FINISHED อัตโนมัติ
// 2. เช็คเครื่องที่ซักเสร็จครบ ResetDelaySeconds แล้ว -> รีเซ็ตกลับเป็น AVAILABLE อัตโนมัติ
func StartAutoFinishWorker() {
	go func() {
		log.Println("⚡ [Background Worker] ทำงานแล้ว (Auto-Finish & Auto-Reset ทุก 2 วินาที)...")
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()

			// ----------------------------------------------------
			// ส่วนที่ 1: ตรวจหาเครื่องที่ครบกำหนดเวลาซัก (RUNNING -> FINISHED)
			// ----------------------------------------------------
			var expiredSessions []models.UsageSession
			err := database.DB.Where("status = ? AND finish_at <= ?", "RUNNING", now).Find(&expiredSessions).Error
			if err == nil {
				for _, session := range expiredSessions {
					compTime := now
					session.Status = "COMPLETED"
					session.CompletedAt = &compTime
					database.DB.Save(&session)

					var machine models.Machine
					if err := database.DB.First(&machine, session.MachineID).Error; err == nil {
						if machine.Status == "IN_USE" {
							machine.Status = "FINISHED"
							database.DB.Save(&machine)
							log.Printf("🎉 [Auto-Finish] เครื่องหมายเลข #%d (%s) ซักเสร็จแล้ว! เปลี่ยนสถานะเป็น FINISHED อัตโนมัติ (User ID: %d)\n", machine.ID, machine.Name, session.UserID)

							// ส่งแจ้งเตือน Real-time ไปยัง Notification Service ทันทีเมื่อซักเสร็จ
							SendNotificationRealtime(session.UserID, machine.ID, fmt.Sprintf("เครื่องซักผ้าหมายเลข %d ซักเสร็จเรียบร้อยแล้ว กรุณานำผ้าออก", machine.ID), "FINISHED")
						}
					}
				}
			}

			// ----------------------------------------------------
			// ส่วนที่ 2: ตรวจหาเครื่องที่ซักเสร็จแล้วเกิน ResetDelaySeconds (FINISHED -> AVAILABLE)
			// ----------------------------------------------------
			var finishedMachines []models.Machine
			if err := database.DB.Where("status = ?", "FINISHED").Find(&finishedMachines).Error; err == nil {
				for _, machine := range finishedMachines {
					var lastSession models.UsageSession
					if err := database.DB.Where("machine_id = ? AND status = ?", machine.ID, "COMPLETED").Order("id desc").First(&lastSession).Error; err == nil {
						if lastSession.CompletedAt != nil {
							elapsed := now.Sub(*lastSession.CompletedAt)
							if elapsed >= time.Duration(ResetDelaySeconds)*time.Second {
								machine.Status = "AVAILABLE"
								database.DB.Save(&machine)
								log.Printf("🔄 [Auto-Reset] เครื่องหมายเลข #%d (%s) ครบ %d วินาทีหลังซักเสร็จ! รีเซ็ตกลับเป็น AVAILABLE อัตโนมัติ\n", machine.ID, machine.Name, ResetDelaySeconds)
							}
						}
					}
				}
			}
		}
	}()
}
