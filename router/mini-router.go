package router

import (
	"encoding/json"
	"fmt"
	"go-file/middleware"
	"go-file/model"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin" // 纯 Go 实现，无需 CGO
	"gorm.io/gorm"
	gonanoid "github.com/matoous/go-nanoid/v2"
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
				var user model.WeChatUser
				// 1. 查找或创建用户
				model.DB.Where(model.WeChatUser{OpenID: wxRes.OpenID}).FirstOrCreate(&user)
				if info.NickName != "" && info.NickName != "微信用户" {
					user.NickName = info.NickName
					user.AvatarURL = info.AvatarUrl
					user.LastLogin = time.Now()
					model.DB.Save(&user)
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
							OpenID:        wxRes.OpenID,
							NanoID:        pathNanoID,
							BrowseURL:     browseURL,
							GraphicRecord: "", // 图形记录先留空，后续根据需求填充
							ViewTime:      time.Now(),
						}
						model.DB.Create(&record)
					}
				}

				c.JSON(200, gin.H{"openid": user.OpenID, "user": user})
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
				BrowseURL    string `json:"browse_url"`    // 新增：浏览URL
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
	}
}
