package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"machine-service/database"
	"machine-service/models"

	"github.com/gin-gonic/gin"
)

// getDefaultUserName คืนค่าชื่อผู้ใช้เริ่มต้นตาม User ID (เช่น ID 2 คือ ชนะภพ อินทะนัย ตามฐานข้อมูลของเพื่อน)
func getDefaultUserName(uid uint) string {
	switch uid {
	case 1:
		return "ผู้ใช้ทั่วไป (ID 1)"
	case 2:
		return "ชนะภพ อินทะนัย"
	default:
		return fmt.Sprintf("ผู้ใช้ ID #%d", uid)
	}
}

// getDefaultUserEmail คืนค่าอีเมลเริ่มต้นตาม User ID (เช่น ID 2 คือ test2@gmail.com)
func getDefaultUserEmail(uid uint) string {
	switch uid {
	case 1:
		return "test1@gmail.com"
	case 2:
		return "test2@gmail.com"
	default:
		return fmt.Sprintf("user%d@gmail.com", uid)
	}
}

// extractUserFromJWT ถอดรหัส Payload ของ JWT Token จาก Header Authorization หรือ Token
func extractUserFromJWT(tokenStr string) (uint, string, string) {
	if tokenStr == "" {
		return 0, "", ""
	}
	parts := strings.Split(tokenStr, " ")
	token := tokenStr
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		token = parts[1]
	}
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) < 2 {
		return 0, "", ""
	}
	payloadSegment := tokenParts[1]
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadSegment)
	if err != nil {
		payloadBytes, err = base64.URLEncoding.DecodeString(payloadSegment)
		if err != nil {
			return 0, "", ""
		}
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return 0, "", ""
	}

	log.Printf("🔑 [JWT] แกะ Payload Token ได้: %+v\n", claims)

	var uid uint
	if v, ok := claims["id"]; ok && v != nil {
		uid = parseFlexibleUint(v)
	} else if v, ok := claims["user_id"]; ok && v != nil {
		uid = parseFlexibleUint(v)
	} else if v, ok := claims["userId"]; ok && v != nil {
		uid = parseFlexibleUint(v)
	} else if v, ok := claims["sub"]; ok && v != nil {
		uid = parseFlexibleUint(v)
	}

	var name, email string
	for _, k := range []string{"name", "user_name", "username", "userName", "fullName", "full_name", "displayName"} {
		if n, ok := claims[k].(string); ok && n != "" {
			name = n
			break
		}
	}
	for _, k := range []string{"email", "user_email", "mail", "userEmail"} {
		if em, ok := claims[k].(string); ok && em != "" {
			email = em
			break
		}
	}

	if uid == 0 || name == "" || email == "" {
		for _, uKey := range []string{"user", "currentUser", "profile", "data"} {
			if uObj, ok := claims[uKey].(map[string]interface{}); ok {
				if uid == 0 {
					if idVal, ok := uObj["id"]; ok {
						uid = parseFlexibleUint(idVal)
					} else if idVal, ok := uObj["user_id"]; ok {
						uid = parseFlexibleUint(idVal)
					}
				}
				if name == "" {
					for _, k := range []string{"name", "user_name", "username", "userName"} {
						if n, ok := uObj[k].(string); ok && n != "" {
							name = n
							break
						}
					}
				}
				if email == "" {
					for _, k := range []string{"email", "user_email", "mail"} {
						if em, ok := uObj[k].(string); ok && em != "" {
							email = em
							break
						}
					}
				}
			}
		}
	}

	// ถ้าได้ User ID แล้ว แต่ใน Token ไม่มี name/email ให้ดึงค่าที่แมปตามข้อมูลสมาชิก
	if uid > 0 {
		if name == "" {
			name = getDefaultUserName(uid)
		}
		if email == "" {
			email = getDefaultUserEmail(uid)
		}
	}

	return uid, name, email
}

func parseFlexibleUint(val interface{}) uint {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return uint(v)
	case float32:
		return uint(v)
	case int:
		return uint(v)
	case int64:
		return uint(v)
	case uint:
		return v
	case string:
		var n uint64
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil {
			return uint(n)
		}
	}
	return 0
}

// GET /machines (รองรับ ?status=AVAILABLE เป็นต้น)
func GetMachines(c *gin.Context) {
	var machines []models.Machine
	query := database.DB.Order("id asc")

	status := c.Query("status")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Find(&machines)

	type MachineResponse struct {
		ID               uint       `json:"id"`
		Name             string     `json:"name"`
		Status           string     `json:"status"`
		UserID           *uint      `json:"user_id,omitempty"`
		UserName         *string    `json:"user_name,omitempty"`
		UserEmail        *string    `json:"user_email,omitempty"`
		RemainingSeconds *int       `json:"remaining_seconds,omitempty"`
		RemainingMinutes *int       `json:"remaining_minutes,omitempty"`
		FinishAt         *time.Time `json:"finish_at,omitempty"`
	}

	now := time.Now()
	res := make([]MachineResponse, 0, len(machines))
	for _, m := range machines {
		item := MachineResponse{ID: m.ID, Name: m.Name, Status: m.Status}
		if m.Status == "IN_USE" {
			var session models.UsageSession
			if err := database.DB.Where("machine_id = ? AND status = ?", m.ID, "RUNNING").Order("id desc").First(&session).Error; err == nil {
				remSec := int(session.FinishAt.Sub(now).Seconds())
				if remSec < 0 {
					remSec = 0
				}
				remMin := (remSec + 59) / 60
				item.UserID = &session.UserID

				uName := session.UserName
				if uName == "" {
					uName = getDefaultUserName(session.UserID)
				}
				uEmail := session.UserEmail
				if uEmail == "" {
					uEmail = getDefaultUserEmail(session.UserID)
				}

				item.UserName = &uName
				item.UserEmail = &uEmail
				item.RemainingSeconds = &remSec
				item.RemainingMinutes = &remMin
				item.FinishAt = &session.FinishAt
			}
		} else if m.Status == "FINISHED" {
			var session models.UsageSession
			if err := database.DB.Where("machine_id = ? AND status = ?", m.ID, "COMPLETED").Order("id desc").First(&session).Error; err == nil && session.CompletedAt != nil {
				remResetSec := ResetDelaySeconds - int(now.Sub(*session.CompletedAt).Seconds())
				if remResetSec < 0 {
					remResetSec = 0
				}
				item.RemainingSeconds = &remResetSec
				item.UserID = &session.UserID

				uName := session.UserName
				if uName == "" {
					uName = getDefaultUserName(session.UserID)
				}
				uEmail := session.UserEmail
				if uEmail == "" {
					uEmail = getDefaultUserEmail(session.UserID)
				}

				item.UserName = &uName
				item.UserEmail = &uEmail
			}
		}
		res = append(res, item)
	}

	c.JSON(http.StatusOK, res)
}

// GET /machines/:id
func GetMachineByID(c *gin.Context) {
	id := c.Param("id")
	var machine models.Machine
	if err := database.DB.First(&machine, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Machine not found"})
		return
	}

	res := gin.H{
		"id":     machine.ID,
		"name":   machine.Name,
		"status": machine.Status,
	}

	// หากเครื่องกำลังทำงาน ให้คำนวณเวลาที่เหลือและดึงข้อมูล Session ล่าสุด
	if machine.Status == "IN_USE" {
		var session models.UsageSession
		if err := database.DB.Where("machine_id = ? AND status = ?", machine.ID, "RUNNING").Order("id desc").First(&session).Error; err == nil {
			now := time.Now()
			remSec := int(session.FinishAt.Sub(now).Seconds())
			if remSec < 0 {
				remSec = 0
			}
			remMin := (remSec + 59) / 60
			res["remaining_seconds"] = remSec
			res["remaining_minutes"] = remMin
			res["user_id"] = session.UserID

			uName := session.UserName
			if uName == "" {
				uName = getDefaultUserName(session.UserID)
			}
			uEmail := session.UserEmail
			if uEmail == "" {
				uEmail = getDefaultUserEmail(session.UserID)
			}

			res["user_name"] = uName
			res["user_email"] = uEmail
			res["started_at"] = session.StartedAt
			res["finish_at"] = session.FinishAt
		}
	} else if machine.Status == "FINISHED" {
		var session models.UsageSession
		if err := database.DB.Where("machine_id = ? AND status = ?", machine.ID, "COMPLETED").Order("id desc").First(&session).Error; err == nil && session.CompletedAt != nil {
			now := time.Now()
			remResetSec := ResetDelaySeconds - int(now.Sub(*session.CompletedAt).Seconds())
			if remResetSec < 0 {
				remResetSec = 0
			}
			res["remaining_reset_seconds"] = remResetSec
			res["user_id"] = session.UserID

			uName := session.UserName
			if uName == "" {
				uName = getDefaultUserName(session.UserID)
			}
			uEmail := session.UserEmail
			if uEmail == "" {
				uEmail = getDefaultUserEmail(session.UserID)
			}

			res["user_name"] = uName
			res["user_email"] = uEmail
		}
	}

	c.JSON(http.StatusOK, res)
}

// POST /machines
func CreateMachine(c *gin.Context) {
	var input struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'name' is required"})
		return
	}

	machine := models.Machine{Name: input.Name, Status: "AVAILABLE"}
	database.DB.Create(&machine)

	c.JSON(http.StatusCreated, gin.H{"message": "Machine created successfully"})
}

// PUT /machines/:id
func UpdateMachine(c *gin.Context) {
	id := c.Param("id")
	var machine models.Machine
	if err := database.DB.First(&machine, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Machine not found"})
		return
	}

	var input struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if input.Name != "" {
		machine.Name = input.Name
	}
	if input.Status != "" {
		machine.Status = input.Status
	}
	database.DB.Save(&machine)

	c.JSON(http.StatusOK, gin.H{"message": "Machine updated successfully"})
}

// DELETE /machines/:id
func DeleteMachine(c *gin.Context) {
	id := c.Param("id")
	var machine models.Machine
	if err := database.DB.First(&machine, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Machine not found"})
		return
	}

	database.DB.Delete(&machine)
	c.JSON(http.StatusOK, gin.H{"message": "Machine deleted successfully"})
}

// POST /machines/:id/start
func StartMachine(c *gin.Context) {
	id := c.Param("id")
	var machine models.Machine
	if err := database.DB.First(&machine, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Machine not found"})
		return
	}

	// รองรับทั้ง JSON Body, Query param, Header และรองรับทั้ง user_id, userId, id (ทั้ง int และ string)
	var rawBody map[string]interface{}
	_ = c.ShouldBindJSON(&rawBody)
	if rawBody == nil {
		rawBody = make(map[string]interface{})
	}

	log.Printf("🔍 [DEBUG] ได้รับ Request เริ่มซักเครื่อง #%s | ข้อมูล Body ที่เพื่อนส่งมา: %+v | Query: %s\n", id, rawBody, c.Request.URL.RawQuery)

	var targetUserID uint

	// 1. ตรวจสอบจาก JSON Body โดยตรง
	if val, ok := rawBody["user_id"]; ok && val != nil {
		targetUserID = parseFlexibleUint(val)
	} else if val, ok := rawBody["userId"]; ok && val != nil {
		targetUserID = parseFlexibleUint(val)
	} else if val, ok := rawBody["id"]; ok && val != nil {
		targetUserID = parseFlexibleUint(val)
	}

	// 1.1 ตรวจสอบกรณีเพื่อนส่ง Object ซ้อนมา เช่น {"user": {"id": 2}} หรือ {"currentUser": {"id": 2}}
	if targetUserID == 0 {
		for _, key := range []string{"user", "currentUser", "profile", "data"} {
			if uObj, ok := rawBody[key].(map[string]interface{}); ok {
				if uid, ok := uObj["id"]; ok && uid != nil {
					targetUserID = parseFlexibleUint(uid)
					break
				} else if uid, ok := uObj["user_id"]; ok && uid != nil {
					targetUserID = parseFlexibleUint(uid)
					break
				}
			}
		}
	}

	// 2. ถ้าใน JSON ไม่มี ตรวจสอบจาก Form หรือ Query param (?user_id=... หรือ ?userId=...)
	if targetUserID == 0 {
		if q := c.Query("user_id"); q != "" {
			targetUserID = parseFlexibleUint(q)
		} else if q := c.Query("userId"); q != "" {
			targetUserID = parseFlexibleUint(q)
		} else if q := c.PostForm("user_id"); q != "" {
			targetUserID = parseFlexibleUint(q)
		} else if q := c.PostForm("userId"); q != "" {
			targetUserID = parseFlexibleUint(q)
		}
	}

	// 3. ตรวจสอบจาก Header (เช่น X-User-Id หรือ User-Id)
	if targetUserID == 0 {
		if h := c.GetHeader("X-User-Id"); h != "" {
			targetUserID = parseFlexibleUint(h)
		} else if h := c.GetHeader("User-Id"); h != "" {
			targetUserID = parseFlexibleUint(h)
		}
	}

	// 3.1 ตรวจสอบจาก JWT Token ใน Header (Authorization: Bearer ..., token, x-access-token)
	var jwtName, jwtEmail string
	if targetUserID == 0 {
		for _, hKey := range []string{"Authorization", "token", "Token", "x-access-token"} {
			if tokenVal := c.GetHeader(hKey); tokenVal != "" {
				jwtUid, jName, jEmail := extractUserFromJWT(tokenVal)
				if jwtUid > 0 {
					targetUserID = jwtUid
					jwtName = jName
					jwtEmail = jEmail
					log.Printf("🔑 [JWT] แกะ Token สำเร็จ! ได้ User ID: %d, Name: %s, Email: %s\n", jwtUid, jName, jEmail)
					break
				}
			}
		}
	}
	// หรือถ้า Token ส่งมาใน JSON Body
	if targetUserID == 0 {
		if tVal, ok := rawBody["token"].(string); ok && tVal != "" {
			jwtUid, jName, jEmail := extractUserFromJWT(tVal)
			if jwtUid > 0 {
				targetUserID = jwtUid
				jwtName = jName
				jwtEmail = jEmail
			}
		}
	}

	// 4. ถ้าไม่มีส่งมาจริงๆ ค่อย default เป็น 1
	if targetUserID == 0 {
		targetUserID = 1
	}

	// สกัดชื่อ (name) และอีเมล (email)
	var targetUserName = jwtName
	var targetUserEmail = jwtEmail

	if val, ok := rawBody["name"].(string); ok {
		targetUserName = val
	} else if val, ok := rawBody["user_name"].(string); ok {
		targetUserName = val
	} else if val, ok := rawBody["userName"].(string); ok {
		targetUserName = val
	}

	if val, ok := rawBody["email"].(string); ok {
		targetUserEmail = val
	} else if val, ok := rawBody["user_email"].(string); ok {
		targetUserEmail = val
	} else if val, ok := rawBody["userEmail"].(string); ok {
		targetUserEmail = val
	}

	if hName := c.GetHeader("X-User-Name"); hName != "" && targetUserName == "" {
		targetUserName = hName
	}
	if hEmail := c.GetHeader("X-User-Email"); hEmail != "" && targetUserEmail == "" {
		targetUserEmail = hEmail
	}

	for _, key := range []string{"user", "currentUser", "profile", "data"} {
		if uObj, ok := rawBody[key].(map[string]interface{}); ok {
			if targetUserName == "" {
				if n, ok := uObj["name"].(string); ok {
					targetUserName = n
				} else if n, ok := uObj["user_name"].(string); ok {
					targetUserName = n
				}
			}
			if targetUserEmail == "" {
				if em, ok := uObj["email"].(string); ok {
					targetUserEmail = em
				} else if em, ok := uObj["user_email"].(string); ok {
					targetUserEmail = em
				}
			}
		}
	}

	// ถ้ายังไม่มีชื่อหรืออีเมล แต่มี User ID ให้ดึงค่าเริ่มต้นตาม ID
	if targetUserName == "" && targetUserID > 0 {
		targetUserName = getDefaultUserName(targetUserID)
	}
	if targetUserEmail == "" && targetUserID > 0 {
		targetUserEmail = getDefaultUserEmail(targetUserID)
	}

	var durationMinutes int
	var durationSeconds int

	if val, ok := rawBody["duration_seconds"]; ok && val != nil {
		durationSeconds = int(parseFlexibleUint(val))
	} else if val, ok := rawBody["durationSeconds"]; ok && val != nil {
		durationSeconds = int(parseFlexibleUint(val))
	}

	if val, ok := rawBody["duration"]; ok && val != nil {
		durationMinutes = int(parseFlexibleUint(val))
	}

	// ถ้าไม่ได้ระบุเวลามา ให้ใช้ค่าเริ่มต้นเป็น 40 วินาที (ไม่รอนานตอนทดสอบ/นำเสนอ)
	if durationMinutes <= 0 && durationSeconds <= 0 {
		durationSeconds = 40
	}

	// ยอมให้เริ่มซักได้ถ้าเครื่องว่าง (AVAILABLE) หรือซักรอบเก่าเสร็จแล้ว (FINISHED)
	if machine.Status != "AVAILABLE" && machine.Status != "FINISHED" {
		c.JSON(http.StatusConflict, gin.H{"error": "Machine is currently not available (เครื่องกำลังใช้งานอยู่ หรือกำลังแจ้งซ่อม)"})
		return
	}

	// เริ่มต้นทำงาน: คำนวณเวลาสิ้นสุด
	now := time.Now()
	var finishAt time.Time

	if durationSeconds > 0 {
		finishAt = now.Add(time.Duration(durationSeconds) * time.Second)
		durationMinutes = (durationSeconds + 59) / 60
	} else {
		finishAt = now.Add(time.Duration(durationMinutes) * time.Minute)
	}

	machine.Status = "IN_USE"
	database.DB.Save(&machine)

	session := models.UsageSession{
		UserID:    targetUserID,
		UserName:  targetUserName,
		UserEmail: targetUserEmail,
		MachineID: machine.ID,
		StartedAt: now,
		FinishAt:  finishAt,
		Status:    "RUNNING",
	}
	database.DB.Create(&session)

	log.Printf("📥 [StartMachine] เครื่อง #%d เริ่มทำงานแล้ว! โดย User ID: %d (ชื่อ: %s, อีเมล: %s, เวลา: %d วินาที, finish_at: %s)\n", machine.ID, targetUserID, targetUserName, targetUserEmail, durationSeconds, finishAt.Format("15:04:05"))

	// ส่งแจ้งเตือน Real-time เมื่อเครื่องเริ่มทำงาน
	SendNotificationRealtime(targetUserID, machine.ID, fmt.Sprintf("เครื่องซักผ้าหมายเลข %d เริ่มทำงานแล้ว (ระยะเวลา %d วินาที)", machine.ID, durationSeconds), "STARTED")

	resData := gin.H{
		"message":          "Machine started successfully",
		"machine_id":       machine.ID,
		"user_id":          targetUserID,
		"user_name":        targetUserName,
		"user_email":       targetUserEmail,
		"status":           machine.Status,
		"duration":         durationMinutes,
		"duration_seconds": durationSeconds,
		"finish_at":        finishAt,
	}

	c.JSON(http.StatusOK, resData)
}

// POST /machines/:id/finish
func FinishMachine(c *gin.Context) {
	id := c.Param("id")
	var machine models.Machine
	if err := database.DB.First(&machine, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Machine not found"})
		return
	}

	if machine.Status != "IN_USE" {
		c.JSON(http.StatusConflict, gin.H{"error": "Machine is not currently in use"})
		return
	}

	// สิ้นสุดการทำงาน
	now := time.Now()
	machine.Status = "FINISHED"
	database.DB.Save(&machine)

	// อัปเดต Session ล่าสุดของเครื่องนี้
	var session models.UsageSession
	if err := database.DB.Where("machine_id = ? AND status = ?", machine.ID, "RUNNING").Order("id desc").First(&session).Error; err == nil {
		session.CompletedAt = &now
		session.Status = "COMPLETED"
		database.DB.Save(&session)

		// แจ้งเตือนเมื่อมีการกดหยุดการทำงาน
		SendNotificationRealtime(session.UserID, machine.ID, fmt.Sprintf("เครื่องซักผ้าหมายเลข %d ถูกสั่งหยุดการทำงาน", machine.ID), "STOPPED")
	}

	c.JSON(http.StatusOK, gin.H{
		"machine_id": machine.ID,
		"status":     machine.Status,
	})
}

// GET /machines/:id/sessions
// หน้าที่: ใช้เก็บประวัติการใช้งานเครื่องซักผ้า และตรวจสอบย้อนหลัง
func GetMachineSessions(c *gin.Context) {
	id := c.Param("id")
	var sessions []models.UsageSession
	database.DB.Where("machine_id = ?", id).Order("id desc").Find(&sessions)
	c.JSON(http.StatusOK, sessions)
}

// GET /sessions
// หน้าที่: ดึงประวัติทั้งหมด รองรับการกรองตาม ?status=..., ?machine_id=..., ?user_id=...
// เพื่อให้ Notification Service และ Auth Service ดึงข้อมูลไปใช้งานได้อย่างสะดวก
func GetAllSessions(c *gin.Context) {
	var sessions []models.UsageSession
	query := database.DB.Order("id desc")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if machineID := c.Query("machine_id"); machineID != "" {
		query = query.Where("machine_id = ?", machineID)
	}
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	query.Find(&sessions)
	c.JSON(http.StatusOK, sessions)
}
