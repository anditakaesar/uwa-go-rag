package worker

import (
	"bytes"
	"context"
	"fmt"
	"image"

	_ "image/jpeg"
	_ "image/png"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/chai2010/webp"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// adapter
type FileService interface {
	Get(ctx context.Context, fileID uuid.UUID) (*domain.File, error)
}

type ThumbnailWorkerArgs struct {
	ID uuid.UUID `json:"id"`
}

func (ThumbnailWorkerArgs) Kind() string { return "Generate-Thumbnail" }

type ThumbnailWorker struct {
	fileSvc       FileService
	storageClient StorageClient
	river.WorkerDefaults[ThumbnailWorkerArgs]
}

type ThumbnailWorkerDep struct {
	FileService   FileService
	StorageClient StorageClient
}

func NewThumbnailWorker(dep ThumbnailWorkerDep) *ThumbnailWorker {
	return &ThumbnailWorker{
		fileSvc:       dep.FileService,
		storageClient: dep.StorageClient,
	}
}

func bytesToImage(buff []byte) (image.Image, string, error) {
	img, format, err := image.Decode(bytes.NewReader(buff))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	// format returns the extension type, e.g., "jpeg", "png", "webp"
	return img, format, nil
}

func (w *ThumbnailWorker) Work(ctx context.Context, job *river.Job[ThumbnailWorkerArgs]) error {
	file, err := w.fileSvc.Get(ctx, job.Args.ID)
	if err != nil {
		return err
	}

	imgBuff, err := w.storageClient.GetObjectIntoBuffer(ctx, file.S3Key)
	if err != nil {
		return err
	}

	srcImg, _, err := bytesToImage(imgBuff)
	if err != nil {
		return err
	}

	originalBounds := srcImg.Bounds()

	maxDim := 300.0
	width, height := float64(originalBounds.Dx()), float64(originalBounds.Dy())
	scale := min(maxDim/width, maxDim/height)
	if scale > 1.0 {
		scale = 1.0 // Don't upscale small images
	}
	targetWidth := max(1, int(width*scale))
	targetHeight := max(1, int(height*scale))

	newRect := image.Rect(0, 0, targetWidth, targetHeight)
	dstImg := image.NewRGBA(newRect)
	draw.BiLinear.Scale(dstImg, newRect, srcImg, originalBounds, draw.Over, nil)

	var webpBuff bytes.Buffer
	webpOptions := &webp.Options{Lossless: false, Quality: 80}

	err = webp.Encode(&webpBuff, dstImg, webpOptions)
	if err != nil {
		return err
	}

	err = w.storageClient.UploadObject(ctx, file.GeneratePublicThumbnailKey(), "image/webp", webpBuff.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (w *ThumbnailWorker) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5}
}
