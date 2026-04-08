package audit

import "testing"

func TestEventNormalizePopulatesDefaults(t *testing.T) {
	event := Event{
		EventType: "user.registered",
		Category:  CategoryAudit,
		Service:   "auth",
		Status:    StatusSuccess,
		Message:   "user registered",
	}.Normalize()

	if event.EventID == "" {
		t.Fatal("expected event id to be generated")
	}
	if event.OccurredAt == "" {
		t.Fatal("expected occurred_at to be generated")
	}
	if event.Metadata == nil {
		t.Fatal("expected metadata map to be initialized")
	}
}

func TestEventValidateRejectsMissingFields(t *testing.T) {
	err := (Event{}).Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
