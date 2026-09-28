package database

import (
	"fmt"
	"log"
	"machine-service/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	database, err := gorm.Open(sqlite.Open("machine.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("ไม่สามารถเชื่อมต่อฐานข้อมูลได้:", err)
	}

	// สร้างตารางอัตโนมัติ
	err = database.AutoMigrate(&models.Machine{}, &models.UsageSession{})
	if err != nil {
		log.Fatal("AutoMigrate ล้มเหลว:", err)
	}

	// Seed ข้อมูลเครื่องให้ครบ 9 เครื่อง (Machine 1 ถึง Machine 9) ถ้ายังไม่ครบ
	var count int64
	database.Model(&models.Machine{}).Count(&count)
	if count < 9 {
		for i := count + 1; i <= 9; i++ {
			database.Create(&models.Machine{
				Name:   fmt.Sprintf("Machine %d", i),
				Status: "AVAILABLE",
			})
		}
	}

	DB = database
}
