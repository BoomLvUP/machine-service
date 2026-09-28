package models

import "time"

type Machine struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(100);not null"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:'AVAILABLE'"` // AVAILABLE, IN_USE, FINISHED, MAINTENANCE
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UsageSession struct {
	ID          uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      uint       `json:"user_id" gorm:"not null"`
	UserName    string     `json:"user_name,omitempty" gorm:"type:varchar(100)"`  // ชื่อผู้ใช้
	UserEmail   string     `json:"user_email,omitempty" gorm:"type:varchar(100)"` // อีเมลผู้ใช้
	MachineID   uint       `json:"machine_id" gorm:"not null"`
	StartedAt   time.Time  `json:"started_at"`
	FinishAt    time.Time  `json:"finish_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `json:"status" gorm:"type:varchar(20);default:'RUNNING'"` // RUNNING, COMPLETED, CANCELLED
}
