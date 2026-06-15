package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

func TestUpdateHandler_Handle(t *testing.T) {
	tests := []struct {
		name              string
		raw               domain.RawUpdate
		processed         domain.ProcessedUpdate
		processorOK       bool
		publishErr        error
		expectedPublished bool
		wantErr           bool
		errContains       string
	}{
		{
			name: "filtered update is skipped",
			raw:  domain.RawUpdate{EventID: "raw-filtered"},
			processed: domain.ProcessedUpdate{
				EventID: "processed-filtered",
			},
			processorOK:       false,
			expectedPublished: false,
			wantErr:           false,
		},
		{
			name: "processed update is published",
			raw:  domain.RawUpdate{EventID: "raw-ok"},
			processed: domain.ProcessedUpdate{
				EventID:     "processed-ok",
				URL:         "https://example.com",
				Description: "summary",
				TgChatIDs:   []int64{1, 2},
				Priority:    PriorityMedium,
			},
			processorOK:       true,
			expectedPublished: true,
			wantErr:           false,
		},
		{
			name: "publisher error is wrapped",
			raw:  domain.RawUpdate{EventID: "raw-error"},
			processed: domain.ProcessedUpdate{
				EventID: "processed-error",
			},
			processorOK:       true,
			publishErr:        errors.New("publish down"),
			expectedPublished: true,
			wantErr:           true,
			errContains:       "publish processed update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			processor := mocks.NewMockRawUpdateProcessor(ctrl)
			publisher := mocks.NewMockUpdatePublisher(ctrl)
			processor.EXPECT().
				Process(gomock.Any(), tt.raw).
				Return(tt.processed, tt.processorOK)
			if tt.expectedPublished {
				publisher.EXPECT().
					Publish(gomock.Any(), tt.processed).
					Return(tt.publishErr)
			}
			handler := NewUpdateHandler(processor, publisher)

			err := handler.Handle(context.Background(), tt.raw)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
