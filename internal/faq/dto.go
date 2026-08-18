package faq

import (
	"net/http"
	"strings"
	"time"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/xerror"
	"github.com/google/uuid"
)

// dto
type FAQListResponse struct {
	ID        uuid.UUID `json:"id"`
	Question  string    `json:"question"`
	AskedBy   *int64    `json:"askedBy"`
	Status    string    `json:"status"`
	Answer    string    `json:"answer"`
	CreatedAt time.Time `json:"createdAt"`
}

func FAQToListResponse(f domain.FAQ) FAQListResponse {
	return FAQListResponse{
		ID:        f.ID,
		Question:  f.Question,
		AskedBy:   f.AskedBy,
		Status:    string(f.Status),
		Answer:    f.Answer,
		CreatedAt: f.CreatedAt,
	}
}

func FAQListToResponse(faqs []domain.FAQ) []FAQListResponse {
	list := make([]FAQListResponse, 0, len(faqs))
	for _, f := range faqs {
		list = append(list, FAQToListResponse(f))
	}
	return list
}

type FAQAnswerResponse struct {
	ID         uuid.UUID  `json:"id"`
	Question   string     `json:"question"`
	Answer     string     `json:"answer"`
	Status     string     `json:"status"`
	AnsweredBy *int64     `json:"answeredBy"`
	AnsweredAt *time.Time `json:"answeredAt"`
}

func FAQToAnswerResponse(f *domain.FAQ) FAQAnswerResponse {
	return FAQAnswerResponse{
		ID:         f.ID,
		Question:   f.Question,
		Answer:     f.Answer,
		Status:     string(f.Status),
		AnsweredBy: f.AnsweredBy,
		AnsweredAt: f.AnsweredAt,
	}
}

type AnswerFAQRequest struct {
	Answer string `json:"answer"`
}

type ListFAQRequest struct {
	Status *domain.FAQStatus
}

func parseParam(r *http.Request) (*ListFAQRequest, error) {
	req := ListFAQRequest{}
	statusStr := strings.TrimSpace(r.URL.Query().Get("status"))
	if statusStr != "" {
		status := domain.FAQStatus(statusStr)
		if !status.IsValid() {
			return nil, &xerror.ErrorValidation{Message: "status is not supported"}
		}

		req.Status = &status
	}

	return &req, nil
}

type UpdateFAQRequest struct {
	Status *string `json:"status"`
}

func (req *UpdateFAQRequest) Validate() error {
	if req.Status != nil {
		if strings.TrimSpace(*req.Status) != "" {
			status := domain.FAQStatus(*req.Status)
			if !status.IsValid() {
				return &xerror.ErrorValidation{Message: "status is not supported"}
			}
		}
	}

	return nil
}

func (req *UpdateFAQRequest) ToDomainParam() *domain.UpdateFAQParam {
	var param domain.UpdateFAQParam
	if req.Status != nil {
		newStatus := domain.FAQStatus(*req.Status)
		param.Status = &newStatus
	}

	return &param
}
