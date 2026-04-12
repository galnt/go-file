package main

import (
	"fmt"
	"go-file/model"
	"log"
)

func main() {
	// 初始化数据库
	db, err := model.InitDB()
	if err != nil {
		log.Fatal("数据库初始化失败:", err)
	}

	// 查询所有 CheckInCampaign
	var campaigns []model.CheckInCampaign
	if err := db.Find(&campaigns).Error; err != nil {
		log.Fatal("查询打卡活动失败:", err)
	}

	fmt.Printf("找到 %d 个打卡活动需要迁移\n", len(campaigns))

	successCount := 0
	for _, camp := range campaigns {
		// 检查是否已存在相同ID的Activity（可能已迁移）
		var existing model.Activity
		if err := db.First(&existing, camp.ID).Error; err == nil {
			// 已存在，更新分类和字段
			existing.Category = "checkin"
			existing.Title = camp.Title
			existing.Description = camp.Description
			existing.CoverImage = camp.CoverImage
			existing.StartTime = camp.StartTime
			existing.EndTime = camp.EndTime
			existing.IsActive = camp.IsActive
			existing.CreatedBy = camp.CreatedBy
			existing.CreatedAt = camp.CreatedAt
			// FilePath 如果没有封面图，可以留空或使用 CoverImage
			if existing.FilePath == "" && camp.CoverImage != "" {
				existing.FilePath = camp.CoverImage
			}
			if err := db.Save(&existing).Error; err != nil {
				log.Printf("更新活动 %d 失败: %v", camp.ID, err)
				continue
			}
			successCount++
			fmt.Printf("更新活动 %d: %s\n", camp.ID, camp.Title)
		} else {
		// 不存在，创建新的 Activity
		activity := model.Activity{
			ID:          camp.ID, // 保持相同ID以确保外键关联
			Category:    "checkin",
			Title:       camp.Title,
			Description: camp.Description,
			CoverImage:  camp.CoverImage,
			FilePath:    camp.CoverImage, // 使用封面图作为文件路径
			StartTime:   camp.StartTime,
			EndTime:     camp.EndTime,
			IsActive:    camp.IsActive,
			CreatedBy:   camp.CreatedBy,
			CreatedAt:   camp.CreatedAt,
			// OpenID 字段留空，打卡活动可能不需要
		}
			if err := db.Create(&activity).Error; err != nil {
				log.Printf("创建活动 %d 失败: %v", camp.ID, err)
				continue
			}
			successCount++
			fmt.Printf("创建活动 %d: %s\n", camp.ID, camp.Title)
		}
	}

	fmt.Printf("迁移完成，成功 %d/%d\n", successCount, len(campaigns))

	// 可选：删除 CheckInCampaign 表（谨慎操作）
	// if successCount == len(campaigns) {
	// 	fmt.Println("正在删除 CheckInCampaign 表...")
	// 	if err := db.Migrator().DropTable(&model.CheckInCampaign{}); err != nil {
	// 		log.Fatal("删除表失败:", err)
	// 	}
	// 	fmt.Println("表已删除")
	// }
}