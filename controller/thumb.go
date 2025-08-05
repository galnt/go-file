package controller

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"strings"

	"github.com/nfnt/resize"
)

func generateImageThumbnail(srcPath, dstPath string, maxWidth uint) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 解码图片
	img, format, err := image.Decode(file)
	if err != nil {
		return err
	}

	// 缩放
	thumb := resize.Resize(maxWidth, 0, img, resize.Lanczos3)

	// 创建缩略图文件
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 根据格式编码
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(out, thumb, nil)
	case "png":
		err = png.Encode(out, thumb)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return err
}

func extractVideoThumbnail(srcPath, dstPath string) error {
	// 示例：ffmpeg -i input.mp4 -ss 00:00:01 -vframes 1 output.jpg
	cmd := exec.Command("ffmpeg", "-i", srcPath, "-ss", "00:00:01", "-vframes", "1", dstPath)
	return cmd.Run()
}
