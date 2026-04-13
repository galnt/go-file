package router

import (
	"encoding/json"
	"fmt"
	"go-file/middleware"
	"go-file/model"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin" // 纯 Go 实现，无需 CGO
	gonanoid "github.com/matoous/go-nanoid/v2"
	"gorm.io/gorm"
)

// --- 配置区 ---
const (
	AppID     = "wx7f32b6dd7fe493a1"
	AppSecret = "7d175627f9dc4d272974b59109e6eb9d"
	UploadDir = "./upload"
	JsonDir   = "./json" // 原 Node.js 的 JSON 数据目录
)

var (
	db *gorm.DB
	// dbStatus bool
)

func init() {

	// 确保必要目录存在
	dirs := []string{UploadDir, JsonDir}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			_ = os.Mkdir(d, os.ModePerm)
		}
	}
}

func setMiniRouter(router *gin.Engine) {
	router.Use(middleware.GlobalAPIRateLimit())

	// 静态文件服务 (对应原 Node.js 的 public)
	router.Static("/imgs", "./public/imgs")
	// router.Static("/upload", UploadDir)

	// 读取 JSON 文件的辅助函数
	serveJson := func(fileName string) gin.HandlerFunc {
		return func(c *gin.Context) {
			path := filepath.Join(JsonDir, fileName)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "JSON file not found: " + fileName})
				return
			}
			// 直接作为 JSON 返回
			c.File(path)
		}
	}

	// router.GET("/", func(c *gin.Context) { c.String(200, "Hello World! (Go Version)") })
	router.GET("/goods_data", serveJson("index.json"))
	router.GET("/swipers", serveJson("swipers.json"))
	router.GET("/classfiy", serveJson("classfiy.json"))
	router.GET("/categorys", serveJson("categorys.json"))

	// --- 2. 微信小程序功能接口 ---
	api := router.Group("/mini")
	{
		// 自动登录
		api.POST("/auth/login", func(c *gin.Context) {

			code := c.Query("code")
			var info struct {
				NickName  string `json:"nickName"`
				AvatarUrl string `json:"avatarUrl"`
			}
			c.ShouldBindJSON(&info)

			url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", AppID, AppSecret, code)
			resp, _ := http.Get(url)
			defer resp.Body.Close()
			var wxRes struct {
				OpenID string `json:"openid"`
			}
			json.NewDecoder(resp.Body).Decode(&wxRes)

			if wxRes.OpenID != "" {
				// 使用 User 和 UserAuth 对象，不再使用 WeChatUser
				user, auth, err := model.FindOrCreateUserByWeChat(wxRes.OpenID, "wechat_mp", info.NickName, info.AvatarUrl)
				if err != nil {
					c.JSON(500, gin.H{"error": "用户处理失败: " + err.Error()})
					return
				}

				// 2. 登录时检查 path 参数，写入浏览记录（仅当对应的 Activity 存在时）
				pathNanoID := c.Query("path")
				if pathNanoID != "" {
					var activity model.Activity
					if err := model.DB.Where("nano_id = ?", pathNanoID).First(&activity).Error; err == nil {
						// Activity 存在，记录浏览历史
						// 构建完整URL（实际前端可能只传NanoID，这里按需求记录完整URL）
						browseURL := fmt.Sprintf("http://127.0.0.1:3000/explorer?path=%s", pathNanoID)
						record := model.BrowseHistory{
							OpenID:        wxRes.OpenID, // 仍然使用微信OpenID作为标识
							NanoID:        pathNanoID,
							BrowseURL:     browseURL,
							GraphicRecord: "", // 图形记录先留空，后续根据需求填充
							ViewTime:      time.Now(),
						}
						model.DB.Create(&record)
					}
				}

				// 返回用户信息，兼容原有前端（返回openid和user对象）
				c.JSON(200, gin.H{"openid": auth.Identifier, "user": user})
			} else {
				c.JSON(401, gin.H{"error": "Login failed"})
			}
		})

		// 文件保存
		api.POST("/upload", func(c *gin.Context) {
			file, err := c.FormFile("file")
			if err != nil {
				c.JSON(400, gin.H{"error": "No file"})
				return
			}
			name := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(file.Filename))
			c.SaveUploadedFile(file, filepath.Join(UploadDir, name))
			c.JSON(200, gin.H{"url": "/upload/" + name})
		})

		// 发布接口
		// 2. 接口实现
		api.POST("/activity", func(c *gin.Context) {
			openid := c.PostForm("openid")

			// 根据openid转换为用户ID
			openid = model.GetUserIdByOpenId(openid)

			activityIDStr := c.PostForm("activity_id")

			// --- 场景一：创建主表记录 (有封面图 cover) ---
			coverFile, _ := c.FormFile("cover")
			if coverFile != nil {
				location := c.PostForm("location")
				eventDate := c.PostForm("event_date")

				// 包含大小写、数字、下划线和减号的完整字符集
				alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz-"

				// 生成 12 位格式
				nanoid, err := gonanoid.Generate(alphabet, 12)
				if err != nil {
					fmt.Println("生成失败:", err)
					return
				}

				// 建立专属目录
				// dirName := fmt.Sprintf("%s_%d", location, time.Now().Unix())
				uploadPath := filepath.Join("./upload", nanoid)
				os.MkdirAll(uploadPath, os.ModePerm)

				// 保存封面
				coverName := "cover_" + coverFile.Filename
				c.SaveUploadedFile(coverFile, filepath.Join(uploadPath, coverName))
				dbCoverPath := "/upload/" + nanoid + "/" + coverName

				activity := model.Activity{
					Location:  location,
					EventDate: eventDate,
					FilePath:  dbCoverPath,
					NanoID:    nanoid,
					OpenID:    openid,
				}
				model.DB.Create(&activity)

				// 返回 ID 给前端，用于接下来的多图上传
				c.JSON(200, gin.H{"status": "success", "activity_id": activity.ID, "dir_name": nanoid})
				return
			}

			// --- 场景二：上传从表图片 (有 activity_id 和 photos) ---
			if activityIDStr != "" {
				photoFile, _ := c.FormFile("photos") // 微信 uploadFile 每次传一个
				if photoFile != nil {
					var act model.Activity
					model.DB.First(&act, activityIDStr)

					// 解析原目录（从主表路径中提取，确保存在同一个文件夹）
					uploadPath := filepath.Dir(filepath.Join(".", act.FilePath))

					pName := fmt.Sprintf("detail_%d_%s", time.Now().UnixNano(), photoFile.Filename)
					c.SaveUploadedFile(photoFile, filepath.Join(uploadPath, pName))

					// 写入从表
					photo := model.ActivityPhoto{
						ActivityID: act.ID,
						FilePath:   filepath.Join(filepath.Base(uploadPath), pName), // 这里根据实际静态路径调整
						Uploader:   openid,
						CreatedAt:  time.Now(),
						IsActive:   true,
					}
					// 修正路径格式供前端访问
					photo.FilePath = "/upload/" + filepath.Base(uploadPath) + "/" + pName

					model.DB.Create(&photo)
					c.JSON(200, gin.H{"status": "success"})
					return
				}
			}

			c.JSON(400, gin.H{"error": "无效的上传请求"})
		})

		// 2. 获取我的活动列表接口
		api.GET("/activities", func(c *gin.Context) {
			openid := c.Query("openid")
			var activities []model.Activity
			// 根据 openid 查询，并按 ID 倒序排列（最新发布的在前面）
			model.DB.Where("open_id = ?", openid).Order("id desc").Find(&activities)
			c.JSON(200, activities)
		})

		// 3. 浏览历史列表（关联 Activity 展示活动名称、地点、日期）
		api.GET("/activity/history", func(c *gin.Context) {
			openid := c.Query("openid")
			if openid == "" {
				c.JSON(400, gin.H{"error": "openid 参数不能为空"})
				return
			}

			// 查询该用户的浏览记录，按浏览时间倒序
			var records []model.BrowseHistory
			model.DB.Where("openid = ?", openid).Order("view_time desc").Find(&records)

			// 收集所有 NanoID，批量查询对应 Activity
			nanoIDList := make([]string, 0, len(records))
			for _, r := range records {
				if r.NanoID != "" {
					nanoIDList = append(nanoIDList, r.NanoID)
				}
			}

			var activities []model.Activity
			if len(nanoIDList) > 0 {
				model.DB.Where("nano_id IN ?", nanoIDList).Find(&activities)
			}

			// 建立 NanoID -> Activity 的 Map，方便关联
			activityMap := make(map[string]model.Activity)
			for _, a := range activities {
				activityMap[a.NanoID] = a
			}

			// 组装最终响应：浏览记录 + 对应活动信息
			type HistoryItem struct {
				ID           uint   `json:"id"`
				NanoID       string `json:"nano_id"`
				BrowseURL    string `json:"browse_url"` // 新增：浏览URL
				ViewTime     string `json:"view_time"`
				ActivityName string `json:"activity_name"` // 活动名称（使用Location）
				Location     string `json:"location"`      // 地点
				EventDate    string `json:"event_date"`    // 日期
			}
			var history []HistoryItem
			for _, r := range records {
				item := HistoryItem{
					ID:        r.ID,
					NanoID:    r.NanoID,
					BrowseURL: r.BrowseURL,
					ViewTime:  r.ViewTime.Format("2006-01-02 15:04:05"),
				}
				if act, ok := activityMap[r.NanoID]; ok {
					item.ActivityName = act.Location // 暂时用Location作为活动名称
					item.Location = act.Location
					item.EventDate = act.EventDate
				}
				history = append(history, item)
			}

			c.JSON(200, gin.H{"data": history})
		})

		api.GET("/all_activities", func(c *gin.Context) {
			// 1. 获取分页参数（设置默认值）
			pageStr := c.DefaultQuery("page", "1")
			limitStr := c.DefaultQuery("limit", "10") // 每页 10 条

			page := 1
			limit := 10
			fmt.Sscanf(pageStr, "%d", &page)
			fmt.Sscanf(limitStr, "%d", &limit)

			// 计算偏移量
			offset := (page - 1) * limit

			var list []model.Activity
			var total int64

			// 2. 获取总数（用于前端判断是否加载完）
			model.DB.Model(&model.Activity{}).Count(&total)

			// 3. 执行分页查询：按创建时间倒序
			model.DB.Order("created_at desc").
				Limit(limit).
				Offset(offset).
				Find(&list)

			c.JSON(200, gin.H{
				"data":     list,
				"total":    total,
				"page":     page,
				"has_more": int64(offset+limit) < total, // 是否还有更多
			})
		})

		// 1. 获取活动详情（包含从表照片）
		api.GET("/activity/detail", func(c *gin.Context) {
			id := c.Query("id")
			var activity model.Activity
			var photos []model.ActivityPhoto

			if err := model.DB.First(&activity, id).Error; err != nil {
				c.JSON(404, gin.H{"error": "活动不存在"})
				return
			}
			// 只查询未失效的照片 (IsActive = true)
			model.DB.Where("activity_id = ? AND is_active = ?", id, true).Find(&photos)

			c.JSON(200, gin.H{
				"activity": activity,
				"photos":   photos,
			})
		})

		// 2. 修改活动基础信息
		api.POST("/activity/update", func(c *gin.Context) {
			id := c.PostForm("id")
			location := c.PostForm("location")
			eventDate := c.PostForm("event_date")

			model.DB.Model(&model.Activity{}).Where("id = ?", id).Updates(model.Activity{
				Location:  location,
				EventDate: eventDate,
			})
			c.JSON(200, gin.H{"status": "success"})
		})

		// 3. 软删除照片（置为失效）
		api.POST("/photo/delete", func(c *gin.Context) {
			photoID := c.PostForm("photo_id")
			// 将 IsActive 设置为 false
			model.DB.Model(&model.ActivityPhoto{}).Where("id = ?", photoID).Update("is_active", false)
			c.JSON(200, gin.H{"status": "success"})
		})

		// ==================== 打卡功能 API ====================

		// 获取所有打卡活动列表（分页，按开始时间排序）
		api.GET("/checkin/campaigns", func(c *gin.Context) {
			userID := c.Query("user_id")
			pageStr := c.DefaultQuery("page", "1")
			limitStr := c.DefaultQuery("limit", "20")
			page, _ := strconv.Atoi(pageStr)
			limit, _ := strconv.Atoi(limitStr)
			if page < 1 {
				page = 1
			}
			offset := (page - 1) * limit

			now := time.Now()
			var activities []model.Activity
			var total int64

			// 查询打卡活动（Category='checkin'）
			model.DB.Model(&model.Activity{}).Where("category = ?", "checkin").Count(&total)
			model.DB.Where("category = ?", "checkin").
				Where("is_active = ?", true).
				Order("start_time desc").
				Limit(limit).Offset(offset).
				Find(&activities)

			// 文件在返回给api时,文件路径需要明确域名
			activities[0].FilePath = "http://127.0.0.1:3000" + activities[0].FilePath

			type CampaignItem struct {
				ID           uint      `json:"id"`
				Title        string    `json:"title"`
				Description  string    `json:"description"`
				CoverImage   string    `json:"cover_image"`
				FilePath     string    `json:"file_path"`
				Category     string    `json:"category"`
				StartTime    time.Time `json:"start_time"`
				EndTime      time.Time `json:"end_time"`
				IsActive     bool      `json:"is_active"`
				CreatedBy    string    `json:"created_by"`
				CreatedAt    time.Time `json:"created_at"`
				StatusText   string    `json:"status_text"`
				TaskCount    int       `json:"task_count"`
				CheckedCount int       `json:"checked_count"`
				UserChecked  bool      `json:"user_checked"`
			}

			result := make([]CampaignItem, 0, len(activities))
			for _, act := range activities {
				item := CampaignItem{
					ID:          act.ID,
					Title:       act.Title,
					Description: act.Description,
					CoverImage:  act.CoverImage,
					FilePath:    act.FilePath,
					Category:    act.Category,
					StartTime:   act.StartTime,
					EndTime:     act.EndTime,
					IsActive:    act.IsActive,
					CreatedBy:   act.CreatedBy,
					CreatedAt:   act.CreatedAt,
				}

				// 状态文本
				if now.Before(act.StartTime) {
					item.StatusText = "未开始"
				} else if now.After(act.EndTime) {
					item.StatusText = "已过期"
				} else {
					item.StatusText = "进行中"
				}

				// 任务数（已废弃 CheckInTask 表，直接设为0）
				item.TaskCount = 0

				// 今日打卡人数（去重）
				today := now.Format("2006-01-02")
				var cc int64
				model.DB.Model(&model.CheckInRecord{}).
					Where("campaign_id = ? AND check_date = ?", act.ID, today).
					Distinct("user_id").Count(&cc)
				item.CheckedCount = int(cc)

				// 当前用户今日是否已打卡
				if userID != "" {
					var uc int64
					model.DB.Model(&model.CheckInRecord{}).
						Where("campaign_id = ? AND user_id = ? AND check_date = ?", act.ID, userID, today).
						Count(&uc)
					item.UserChecked = uc > 0
				}

				result = append(result, item)
			}

			c.JSON(200, gin.H{
				"data":     result,
				"total":    total,
				"page":     page,
				"has_more": int64(offset+limit) < total,
			})
		})

		// 获取活动详情（含任务列表和动态）
		api.GET("/checkin/campaign/detail", func(c *gin.Context) {
			idStr := c.Query("id")
			userID := c.Query("user_id")

			// var campaign model.CheckInCampaign
			var activity model.Activity
			if err := model.DB.First(&activity, idStr).Error; err != nil {
				c.JSON(404, gin.H{"error": "活动不存在"})
				return
			}

			// 文件在返回给api时,文件路径需要明确域名
			activity.FilePath = "http://127.0.0.1:3000" + activity.FilePath

			now := time.Now()
			today := now.Format("2006-01-02")

			// 获取任务列表（含每个任务的打卡人数和当前用户是否已打卡）
			// 任务列表（已废弃 CheckInTask 表，返回空数组）
			type TaskItem struct {
				CheckedCount int    `json:"checked_count"`
				UserChecked  bool   `json:"user_checked"`
				StatusText   string `json:"status_text"`
				IsToday      bool   `json:"is_today"`
			}
			taskItems := make([]TaskItem, 0)

			// 获取动态（打卡记录，包含用户信息，最新50条）
			type FeedItem struct {
				model.CheckInRecord
				Nickname   string `json:"nickname"`
				AvatarURL  string `json:"avatar_url"`
				TaskTitle  string `json:"task_title"`
				CheckCount int    `json:"check_count"` // 该用户累计打卡次数
				TaskIndex  int    `json:"task_index"`  // 第几次任务
			}

			var records []model.CheckInRecord
			model.DB.Where("campaign_id = ?", activity.ID).
				Order("created_at desc").Limit(50).Find(&records)

			// 批量拉取用户信息
			userIDs := make([]string, 0)
			for _, r := range records {
				userIDs = append(userIDs, r.UserID)
			}
			var users []model.User
			if len(userIDs) > 0 {
				model.DB.Where("user_id IN ?", userIDs).Find(&users)
			}
			userMap := make(map[string]model.User)
			for _, u := range users {
				userMap[u.UserID] = u
			}

			feeds := make([]FeedItem, 0, len(records))
			for _, r := range records {
				fi := FeedItem{CheckInRecord: r}
				if u, ok := userMap[r.UserID]; ok {
					fi.Nickname = u.Nickname
					fi.AvatarURL = u.AvatarURL
				}
				fi.TaskTitle = "每日打卡"
				// 该用户在此活动的累计打卡次数
				var cnt int64
				model.DB.Model(&model.CheckInRecord{}).
					Where("campaign_id = ? AND user_id = ?", activity.ID, r.UserID).Count(&cnt)
				fi.CheckCount = int(cnt)

				// 任务索引（已废弃 CheckInTask 表，默认设为1）
				fi.TaskIndex = 1

				// 处理图片路径，添加域名前缀
				if r.Images != "" {
					paths := strings.Split(r.Images, ",")
					for i, p := range paths {
						paths[i] = "http://127.0.0.1:3000" + p
					}
					fi.Images = strings.Join(paths, ",")
				} else {
					fi.Images = ""
				}

				feeds = append(feeds, fi)
			}

			// 活动状态
			statusText := "进行中"
			if now.Before(activity.StartTime) {
				statusText = "未开始"
			} else if now.After(activity.EndTime) {
				statusText = "已过期"
			}

			// 参与人数（去重）
			var participantCount int64
			model.DB.Model(&model.CheckInRecord{}).
				Where("campaign_id = ?", activity.ID).
				Distinct("user_id").Count(&participantCount)

			// 当前用户今日是否已打卡
			userChecked := false
			if userID != "" {
				var uc int64
				model.DB.Model(&model.CheckInRecord{}).
					Where("campaign_id = ? AND user_id = ? AND check_date = ?", activity.ID, userID, today).
					Count(&uc)
				userChecked = uc > 0
			}

			c.JSON(200, gin.H{
				"campaign":          activity,
				"status_text":       statusText,
				"tasks":             taskItems,
				"feeds":             feeds,
				"participant_count": participantCount,
				"user_checked":      userChecked,
			})
		})

		// 提交打卡
		api.POST("/checkin/submit", func(c *gin.Context) {
			userID := c.PostForm("user_id")
			campaignIDStr := c.PostForm("campaign_id")
			taskIDStr := c.PostForm("task_id")
			content := c.PostForm("content")
			visibility := c.PostForm("visibility")
			if visibility == "" {
				visibility = "all"
			}

			campaignID, _ := strconv.ParseUint(campaignIDStr, 10, 64)
			taskID, _ := strconv.ParseUint(taskIDStr, 10, 64)

			if userID == "" || campaignID == 0 {
				c.JSON(400, gin.H{"error": "参数不完整"})
				return
			}

			// 检查活动是否存在且在进行中（已废弃 CheckInTask 表）
			var activity model.Activity
			if err := model.DB.First(&activity, campaignID).Error; err != nil || activity.Category != "checkin" {
				c.JSON(404, gin.H{"error": "活动不存在"})
				return
			}
			now := time.Now()
			if now.Before(activity.StartTime) || now.After(activity.EndTime) {
				c.JSON(400, gin.H{"error": "活动不在有效期内"})
				return
			}

			/* 检查用户是否已经为该活动今日打卡（任意任务）
			var existCount int64
			model.DB.Model(&model.CheckInRecord{}).
				Where("campaign_id = ? AND user_id = ? AND check_date = ?", campaignID, userID, now.Format("2006-01-02")).
				Count(&existCount)
			if existCount > 0 {
				c.JSON(400, gin.H{"error": "您今日已打卡，请勿重复提交"})
				return
			}*/

			// 处理上传图片（multipart form，多张图片）
			form, _ := c.MultipartForm()
			var imageURLs []string

			if form != nil && form.File != nil {
				photos := form.File["photos"]
				for _, photo := range photos {
					fileName := fmt.Sprintf("checkin_%d_%d%s", now.UnixNano(), campaignID, filepath.Ext(photo.Filename))
					savePath := filepath.Join(UploadDir, "checkin", fileName)
					os.MkdirAll(filepath.Join(UploadDir, "checkin"), os.ModePerm)
					if err := c.SaveUploadedFile(photo, savePath); err == nil {
						imageURLs = append(imageURLs, "/upload/checkin/"+fileName)
					}
				}
			}

			record := model.CheckInRecord{
				CampaignID: uint(campaignID),
				TaskID:     uint(taskID),
				UserID:     userID,
				Content:    content,
				Images:     strings.Join(imageURLs, ","),
				Visibility: visibility,
				CheckDate:  now.Format("2006-01-02"),
			}
			if err := model.DB.Create(&record).Error; err != nil {
				c.JSON(500, gin.H{"error": "打卡失败: " + err.Error()})
				return
			}

			c.JSON(200, gin.H{"status": "success", "record": record})
		})

		// 打卡排行榜
		api.GET("/checkin/rank", func(c *gin.Context) {
			campaignIDStr := c.Query("campaign_id")
			rankType := c.DefaultQuery("type", "total") // total, week, lastweek, today, yesterday
			userID := c.Query("user_id")

			campaignID, _ := strconv.ParseUint(campaignIDStr, 10, 64)
			if campaignID == 0 {
				c.JSON(400, gin.H{"error": "campaign_id不能为空"})
				return
			}

			now := time.Now()
			var startDate, endDate string

			switch rankType {
			case "today":
				startDate = now.Format("2006-01-02")
				endDate = startDate
			case "yesterday":
				yesterday := now.AddDate(0, 0, -1)
				startDate = yesterday.Format("2006-01-02")
				endDate = startDate
			case "week":
				// 本周：从本周一到今天
				weekday := int(now.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				monday := now.AddDate(0, 0, -(weekday - 1))
				startDate = monday.Format("2006-01-02")
				endDate = now.Format("2006-01-02")
			case "lastweek":
				weekday := int(now.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				lastMonday := now.AddDate(0, 0, -(weekday-1)-7)
				lastSunday := lastMonday.AddDate(0, 0, 6)
				startDate = lastMonday.Format("2006-01-02")
				endDate = lastSunday.Format("2006-01-02")
			default: // total
				startDate = ""
				endDate = ""
			}

			type RankRow struct {
				UserID     string `json:"user_id"`
				CheckCount int    `json:"check_count"`
			}

			var rows []RankRow
			query := model.DB.Model(&model.CheckInRecord{}).
				Select("user_id, COUNT(*) as check_count").
				Where("campaign_id = ?", campaignID).
				Group("user_id").
				Order("check_count desc").
				Limit(100)

			if startDate != "" {
				query = query.Where("check_date >= ? AND check_date <= ?", startDate, endDate)
			}
			query.Scan(&rows)

			// 批量获取用户信息
			userIDs := make([]string, 0, len(rows))
			for _, r := range rows {
				userIDs = append(userIDs, r.UserID)
			}
			var users []model.User
			if len(userIDs) > 0 {
				model.DB.Where("user_id IN ?", userIDs).Find(&users)
			}
			userMap := make(map[string]model.User)
			for _, u := range users {
				userMap[u.UserID] = u
			}

			rankList := make([]model.CheckInRankItem, 0, len(rows))
			myRank := -1
			myCount := 0
			for i, r := range rows {
				item := model.CheckInRankItem{
					UserID:     r.UserID,
					CheckCount: r.CheckCount,
					Rank:       i + 1,
				}
				if u, ok := userMap[r.UserID]; ok {
					item.Nickname = u.Nickname
					item.AvatarURL = u.AvatarURL
				}
				rankList = append(rankList, item)
				if r.UserID == userID {
					myRank = i + 1
					myCount = r.CheckCount
				}
			}

			// 总参与人数
			var participantCount int64
			model.DB.Model(&model.CheckInRecord{}).
				Where("campaign_id = ?", campaignID).
				Distinct("user_id").Count(&participantCount)

			c.JSON(200, gin.H{
				"rank_list":         rankList,
				"my_rank":           myRank,
				"my_count":          myCount,
				"participant_count": participantCount,
			})
		})

		// 我的打卡日记：按月查询（返回该月有打卡的日期列表和详情）
		api.GET("/checkin/my/diary", func(c *gin.Context) {
			userID := c.Query("user_id")
			month := c.Query("month") // 格式：YYYY-MM
			if userID == "" || month == "" {
				c.JSON(400, gin.H{"error": "参数不完整"})
				return
			}

			// 查询该月所有打卡记录
			startDate := month + "-01"
			// 计算月末
			t, _ := time.Parse("2006-01", month)
			endDate := t.AddDate(0, 1, -1).Format("2006-01-02")

			var records []model.CheckInRecord
			model.DB.Where("user_id = ? AND check_date >= ? AND check_date <= ?", userID, startDate, endDate).
				Order("check_date asc, created_at asc").
				Find(&records)

			// 获取关联的活动和任务信息
			campaignIDs := make([]uint, 0)
			taskIDs := make([]uint, 0)
			for _, r := range records {
				campaignIDs = append(campaignIDs, r.CampaignID)
				taskIDs = append(taskIDs, r.TaskID)
			}

			var campaigns []model.CheckInCampaign
			if len(campaignIDs) > 0 {
				model.DB.Where("id IN ?", campaignIDs).Find(&campaigns)
			}
			campaignMap := make(map[uint]model.CheckInCampaign)
			for _, c2 := range campaigns {
				campaignMap[c2.ID] = c2
			}

			type DiaryRecord struct {
				model.CheckInRecord
				CampaignTitle string `json:"campaign_title"`
				CampaignCover string `json:"campaign_cover"`
				TaskTitle     string `json:"task_title"`
			}

			// 按日期分组
			dateMap := make(map[string][]DiaryRecord)
			checkedDates := make([]string, 0)

			for _, r := range records {
				dr := DiaryRecord{CheckInRecord: r}
				if ca, ok := campaignMap[r.CampaignID]; ok {
					dr.CampaignTitle = ca.Title
					dr.CampaignCover = ca.CoverImage
				}

				if _, exists := dateMap[r.CheckDate]; !exists {
					checkedDates = append(checkedDates, r.CheckDate)
				}
				dateMap[r.CheckDate] = append(dateMap[r.CheckDate], dr)
			}

			c.JSON(200, gin.H{
				"checked_dates": checkedDates,
				"diary":         dateMap,
			})
		})

		// 我的打卡日记：查询某天的打卡详情
		api.GET("/checkin/my/day", func(c *gin.Context) {
			userID := c.Query("user_id")
			date := c.Query("date") // 格式：YYYY-MM-DD

			if userID == "" || date == "" {
				c.JSON(400, gin.H{"error": "参数不完整"})
				return
			}

			var records []model.CheckInRecord
			model.DB.Where("user_id = ? AND check_date = ?", userID, date).
				Order("created_at asc").Find(&records)

			// 关联活动和任务信息
			type DayRecord struct {
				model.CheckInRecord
				CampaignTitle string   `json:"campaign_title"`
				CampaignCover string   `json:"campaign_cover"`
				TaskTitle     string   `json:"task_title"`
				ImageList     []string `json:"image_list"`
			}

			var campaignIDs []uint
			var taskIDs []uint
			for _, r := range records {
				campaignIDs = append(campaignIDs, r.CampaignID)
				taskIDs = append(taskIDs, r.TaskID)
			}

			var campaigns []model.CheckInCampaign
			if len(campaignIDs) > 0 {
				model.DB.Where("id IN ?", campaignIDs).Find(&campaigns)
			}
			cMap := make(map[uint]model.CheckInCampaign)
			for _, ca := range campaigns {
				cMap[ca.ID] = ca
			}

			result := make([]DayRecord, 0, len(records))
			for _, r := range records {
				dr := DayRecord{CheckInRecord: r}
				if ca, ok := cMap[r.CampaignID]; ok {
					dr.CampaignTitle = ca.Title
					dr.CampaignCover = ca.CoverImage
				}
				// 解析图片列表
				if r.Images != "" {
					dr.ImageList = strings.Split(r.Images, ",")
				} else {
					dr.ImageList = []string{}
				}
				result = append(result, dr)
			}

			c.JSON(200, gin.H{"data": result})
		})

		// 静态文件服务 - 打卡图片
		api.Static("/checkin-images", filepath.Join(UploadDir, "checkin"))
	}
}
