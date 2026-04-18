package api

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type DialogService struct {
	ctx context.Context
}

func NewDialogService() *DialogService {
	return &DialogService{}
}

func (s *DialogService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *DialogService) PickDirectory(title string) (string, error) {
	return runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
}
