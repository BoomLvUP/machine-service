# 🧺 Machine Service (ระบบจัดการสถานะเครื่องซักผ้า)

> **วิชาสถาปัตยกรรมซอฟต์แวร์ / ไมโครเซอร์วิส (Microservices Architecture)**  
> **ผู้รับผิดชอบระบบ:** นายธนากร ไชยรัตน์ (รหัสนักศึกษา: 67114540244)  
> **พอร์ตการทำงาน (Port):** `8082`

---

## 📖 ภาพรวมระบบ (System Overview)

**Machine Service** เป็นไมโครเซอร์วิสที่ทำหน้าที่จัดการสถานะและการทำงานของเครื่องซักผ้าทั้งหมดในระบบ พร้อมทั้งเก็บประวัติรอบการซักผ้า (Usage Sessions) และเชื่อมต่อกับเซอร์วิสอื่นในระบบไมโครเซอร์วิส:

1. **Auth / User Service (ผู้ใช้):** รับคำสั่งเริ่มซักผ้าจากผู้ใช้งาน โดยรองรับการระบุ `user_id`, `name`, `email`
2. **Notification Service (แจ้งเตือน):** ส่งแจ้งเตือนอัตโนมัติแบบ **Real-time Webhook** ไปยังเซอร์วิสแจ้งเตือน เมื่อเครื่องเริ่มทำงาน หรือซักเสร็จสิ้น
3. **Live Monitoring Dashboard:** มีหน้าเว็บ UI แบบ Interactive แสดงสถานะแอนิเมชันของเครื่องซักผ้าแบบเรียลไทม์

---

## 🏗️ สถาปัตยกรรมระบบ (Microservices Architecture)

```
+-----------------------------------------------------------------------+
|                            Client / Web UI                            |
+-----------------------------------+-----------------------------------+
                                    |
                                    v
+-----------------------+   +-----------------------+   +-----------------------+
|  Auth / User Service  |   |    Machine Service    |   |  Notification Service |
|      (Service 1)      |   |   (Service 2 - เรา)   |   |      (Service 3)      |
|  ชนะภพ อินทะนัย       |   |   ธนากร ไชยรัตน์      |   |   ศรายุทธ สมรักษ์     |
+-----------+-----------+   +-----------+-----------+   +-----------+-----------+
            |                           |                           ^
            |  POST /machines/:id/start |                           |
            +-------------------------->+   Real-time Webhook Push  |
                                        +---------------------------+
                                        (STARTED / FINISHED / STOPPED)
```

---

## 🗄️ โครงสร้างฐานข้อมูล (Database Schema)

ระบบใช้ฐานข้อมูล SQLite ผ่าน GORM โดยแยก Database เป็นอิสระ (`machine_db` / `machine.db`):

### 1. ตาราง `machines` (ข้อมูลเครื่องซักผ้า)
| ฟิลด์ (Field) | ชนิดข้อมูล (Type) | คำอธิบาย |
|---|---|---|
| `id` | `uint` (PK, Auto Increment) | หมายเลขเครื่องซักผ้า (1 - 9) |
| `name` | `varchar(100)` | ชื่อเครื่อง เช่น `Machine 1` |
| `status` | `varchar(20)` | สถานะ: `AVAILABLE`, `IN_USE`, `FINISHED`, `MAINTENANCE` |
| `created_at` | `datetime` | วันที่สร้างข้อมูล |
| `updated_at` | `datetime` | วันที่อัปเดตสถานะล่าสุด |

### 2. ตาราง `usage_sessions` (ประวัติการใช้งาน)
| ฟิลด์ (Field) | ชนิดข้อมูล (Type) | คำอธิบาย |
|---|---|---|
| `id` | `uint` (PK, Auto Increment) | รหัสรอบการซักผ้า |
| `user_id` | `uint` | รหัสผู้ใช้งาน |
| `user_name` | `varchar(100)` | ชื่อผู้ใช้งาน |
| `user_email` | `varchar(100)` | อีเมลผู้ใช้งาน |
| `machine_id` | `uint` | หมายเลขเครื่องซักผ้าที่ใช้ |
| `started_at` | `datetime` | เวลาเริ่มซักผ้า |
| `finish_at` | `datetime` | เวลาที่คาดว่าจะเสร็จ |
| `completed_at` | `datetime` | เวลาที่ซักเสร็จจริง |
| `status` | `varchar(20)` | สถานะรอบการซัก: `RUNNING`, `COMPLETED`, `CANCELLED` |

---

## 📡 รายการ API Endpoints

### 1. การจัดการเครื่องซักผ้า (Machine Endpoints)
| Method | Endpoint | คำอธิบาย | ตัวอย่าง Payload |
|---|---|---|---|
| `GET` | `/health` | ตรวจสอบสถานะ Service | - |
| `GET` | `/machines` | ดูรายการเครื่องซักผ้าทั้งหมด (รองรับ `?status=...`) | - |
| `GET` | `/machines/:id` | ดูข้อมูลเครื่องซักผ้าตาม ID พร้อมเวลานับถอยหลัง | - |
| `POST` | `/machines` | เพิ่มเครื่องซักผ้าเครื่องใหม่ | `{"name": "Machine 10"}` |
| `PUT` | `/machines/:id` | แก้ไขสถานะเครื่อง (เช่น ปรับเป็น `AVAILABLE`) | `{"status": "AVAILABLE"}` |
| `DELETE` | `/machines/:id` | ลบเครื่องซักผ้า | - |

### 2. การควบคุมการซักผ้า (Operation Endpoints)
| Method | Endpoint | คำอธิบาย | ตัวอย่าง Payload |
|---|---|---|---|
| `POST` | `/machines/:id/start` | สั่งให้เครื่องเริ่มทำงาน | `{"user_id": 2, "name": "ชนะภพ", "duration": 40}` |
| `POST` | `/machines/:id/finish` | สั่งหยุดการทำงานทันทีก่อนหมดเวลา | - |

### 3. ประวัติการใช้งาน (Session Endpoints)
| Method | Endpoint | คำอธิบาย |
|---|---|---|
| `GET` | `/machines/:id/sessions` | ดูประวัติการใช้งานเฉพาะเครื่องนั้น ๆ |
| `GET` | `/sessions` | ดูประวัติทั้งหมด (รองรับ `?status=COMPLETED`, `?user_id=...`, `?machine_id=...`) |

### 4. การตั้งค่าระบบแจ้งเตือน (Notification Config)
| Method | Endpoint | คำอธิบาย | ตัวอย่าง Payload |
|---|---|---|---|
| `POST` | `/config/notification-url` | ตั้งค่า URL Webhook ของ Notification Service | `{"url": "https://<ngrok>/notifications"}` |
| `GET` | `/config/notification-url` | ดู URL ปัจจุบันที่ตั้งไว้ | - |
| `GET` | `/notifications/external` | ดึงข้อมูลแจ้งเตือนจาก Notification Service มาแสดงผล | - |

---

## ⚡ คุณสมบัติเด่นของระบบ (Core Features)

1. **Auto-Finish & Auto-Reset Background Worker:**
   - มี Background Goroutine คอยตรวจสอบเวลาเครื่องที่ซักครบกำหนดแบบอัตโนมัติทุก 2 วินาที
   - เปลี่ยนสถานะจาก `IN_USE` เป็น `FINISHED` พร้อมบันทึก `completed_at`
   - เมื่อสถานะ `FINISHED` ค้างไว้ครบ 30 วินาที ระบบจะรีเซ็ตเครื่องกลับเป็น `AVAILABLE` อัตโนมัติ เพื่อพร้อมรับรอบใหม่

2. **Real-time Webhook Notification:**
   - ส่ง HTTP POST ไปยัง Notification Service ทันทีเมื่อเครื่องเริ่มทำงาน (`STARTED`), ซักเสร็จ (`FINISHED`), หรือถูกสั่งหยุด (`STOPPED`)
   - ทำงานแบบ Non-blocking ไม่ทำให้ Service สะดุด

3. **Interactive Live Monitor Dashboard:**
   - เข้าใช้งานได้ผ่านเบราว์เซอร์ที่ `http://localhost:8082` หรือ `/dashboard`
   - มีแอนิเมชันถังซักหมุน เสียงไฟ LED แสดงเวลาที่เหลือ และตารางประวัติการซักที่ดึงสดจากฐานข้อมูล

---

## 🚀 วิธีการติดตั้งและรันระบบ (How to Run)

### ความต้องการพื้นฐาน (Prerequisites)
- [Go (Golang)](https://go.dev/dl/) เวอร์ชัน 1.20 ขึ้นไป

### 1. ดาวน์โหลดและเปิดโปรเจกต์
```bash
git clone <URL_REPOSITORY>
cd machine-service
```

### 2. ดาวน์โหลด Dependencies
```bash
go mod tidy
```

### 3. สั่งรัน Service
รันผ่านคำสั่ง Go:
```bash
go run main.go
```
หรือรันผ่าน Batch Script (บน Windows):
```cmd
run.bat
```

### 4. เข้าใช้งาน
- **Dashboard หน้าเว็บ:** เปิดเบราว์เซอร์ไปที่ [http://localhost:8082](http://localhost:8082)
- **API Base URL:** `http://localhost:8082`

---

## 📂 โครงสร้างโฟลเดอร์ (Directory Structure)

```
machine-service/
├── database/
│   └── db.go                   # เชื่อมต่อ SQLite และ AutoMigrate / Seed ข้อมูล
├── handlers/
│   ├── machine_handler.go      # Handlers หลักสำหรับ API /machines และ /sessions
│   ├── notification_dispatcher.go # ส่งแจ้งเตือน Real-time Webhook ไปยัง Service 3
│   └── worker.go               # Background Worker (Auto-Finish และ Auto-Reset)
├── models/
│   └── machine.go              # Struct โมเดล Machine และ UsageSession
├── static/
│   └── index.html              # หน้าเว็บ Live Monitor Dashboard
├── go.mod                      # Go Module Definition
├── go.sum                      # Go Checksum
├── main.go                     # จุดเริ่มต้นโปรแกรม (Routing, CORS, Server Start)
├── run.bat                     # สคริปต์รันเซิร์ฟเวอร์แบบคลิกเดียว
├── run_ngrok.bat               # สคริปต์เปิด ngrok พอร์ต 8082
├── run.txt                     # คู่มือคำสั่ง curl และตัวอย่างการเชื่อมต่อกับเพื่อน
└── README.md                   # เอกสารประกอบโครงการ
```

---

## 👨‍💻 ข้อมูลผู้พัฒนา (Author)
- **ชื่อ-นามสกุล:** นายธนากร ไชยรัตน์
- **รหัสนักศึกษา:** 67114540244
- **บทบาท:** Machine Service (ระบบจัดการสถานะเครื่องซักผ้า)
