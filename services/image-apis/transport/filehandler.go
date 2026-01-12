package transport

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	apperrors "github.com/zhunismp/imagep-backend/internal/errors"
	"github.com/zhunismp/imagep-backend/services/image-apis/service"
)

type FileProcessor interface {
	Upload(ctx context.Context, taskId string, files []*multipart.FileHeader) (string, error)
	Process(ctx context.Context, taskId string) error
	Download(ctx context.Context, taskId string) (service.ProcessingResult, error)
}

type ProcessingHandler struct {
	frontendHost string
	fp           FileProcessor
}

func NewProcessHandler(fp FileProcessor, feHost string) *ProcessingHandler {
	return &ProcessingHandler{fp: fp, frontendHost: feHost}
}

func (h *ProcessingHandler) Upload(c *fiber.Ctx) error {
	// TODO: move this uuid creation inside service
	// This is just dirty fix
	taskId := c.Params("taskId", uuid.NewString())

	f, err := c.MultipartForm()
	if err != nil {
		c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "something went wrong",
		})
	}

	files := f.File["images"]

	taskId, err = h.fp.Upload(c.Context(), taskId, files)
	if err != nil {
		return mapErr(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"taskId": taskId,
	})

}

func (h *ProcessingHandler) Process(c *fiber.Ctx) error {
	taskId := c.Params("taskId")

	if taskId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid task id"})
	}

	if err := h.fp.Process(c.Context(), taskId); err != nil {
		return mapErr(c, err)
	}

	// redirect user to download
	redirectPath := fmt.Sprintf("%s/downloads/%s", h.frontendHost, taskId)
	return c.Redirect(redirectPath)
}

func (h *ProcessingHandler) Download(c *fiber.Ctx) error {
	taskId := c.Params("taskId")

	if taskId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid task id"})
	}

	res, err := h.fp.Download(c.Context(), taskId)
	if err != nil {
		return mapErr(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(res)
}

func mapErr(c *fiber.Ctx, err error) error {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return c.Status(appErr.MapToHttpCode()).JSON(fiber.Map{"error": appErr.Message})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "something went wrong"})
}
