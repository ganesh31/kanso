package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"kanso/internal/data"
	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
	"kanso/internal/scan"
)

// SystemStore keeps saved systems. Get and Delete of an unknown id return
// an error wrapping market.ErrNotFound. Save assigns a new id to a system
// without one and replaces the system with the same id otherwise.
type SystemStore interface {
	Save(ctx context.Context, sys rules.System) (string, error)
	Get(ctx context.Context, id string) (rules.System, error)
	List(ctx context.Context) ([]rules.System, error)
	Delete(ctx context.Context, id string) error
}

// StoreSystems adapts the SQLite store to SystemStore.
type StoreSystems struct{ S *data.Store }

// Save implements SystemStore.
func (s StoreSystems) Save(ctx context.Context, sys rules.System) (string, error) {
	return s.S.SaveSystem(ctx, sys)
}

// Get implements SystemStore.
func (s StoreSystems) Get(ctx context.Context, id string) (rules.System, error) {
	return s.S.GetSystem(ctx, id)
}

// List implements SystemStore.
func (s StoreSystems) List(ctx context.Context) ([]rules.System, error) { return s.S.ListSystems(ctx) }

// Delete implements SystemStore.
func (s StoreSystems) Delete(ctx context.Context, id string) error { return s.S.DeleteSystem(ctx, id) }

// MemSystems is an in-memory SystemStore, safe for concurrent use.
type MemSystems struct {
	mu      sync.Mutex
	systems map[string]rules.System
}

// NewMemSystems returns an empty in-memory SystemStore.
func NewMemSystems() *MemSystems { return &MemSystems{systems: map[string]rules.System{}} }

// Save implements SystemStore.
func (m *MemSystems) Save(_ context.Context, sys rules.System) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sys.ID == "" {
		sys.ID = uuid.NewString()
	}
	m.systems[sys.ID] = sys
	return sys.ID, nil
}

// Get implements SystemStore.
func (m *MemSystems) Get(_ context.Context, id string) (rules.System, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sys, ok := m.systems[id]
	if !ok {
		return rules.System{}, fmt.Errorf("system %q: %w", id, market.ErrNotFound)
	}
	return sys, nil
}

// List implements SystemStore: every system, sorted by name.
func (m *MemSystems) List(_ context.Context) ([]rules.System, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]rules.System, 0, len(m.systems))
	for _, sys := range m.systems {
		out = append(out, sys)
	}
	slices.SortFunc(out, func(a, b rules.System) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

// Delete implements SystemStore.
func (m *MemSystems) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.systems[id]; !ok {
		return fmt.Errorf("system %q: %w", id, market.ErrNotFound)
	}
	delete(m.systems, id)
	return nil
}

func registerSystems(api huma.API, h *handlers) {
	huma.Register(api, huma.Operation{
		OperationID: "list-systems",
		Method:      http.MethodGet,
		Path:        "/api/systems",
		Summary:     "List saved systems",
	}, h.listSystems)
	huma.Register(api, huma.Operation{
		OperationID:   "create-system",
		Method:        http.MethodPost,
		Path:          "/api/systems",
		Summary:       "Save a new system",
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusBadRequest, http.StatusForbidden},
	}, h.createSystem)
	huma.Register(api, huma.Operation{
		OperationID: "get-system",
		Method:      http.MethodGet,
		Path:        "/api/systems/{id}",
		Summary:     "Get a saved system",
		Errors:      []int{http.StatusNotFound},
	}, h.getSystem)
	huma.Register(api, huma.Operation{
		OperationID: "update-system",
		Method:      http.MethodPut,
		Path:        "/api/systems/{id}",
		Summary:     "Replace a saved system",
		Errors:      []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound},
	}, h.updateSystem)
	huma.Register(api, huma.Operation{
		OperationID:   "delete-system",
		Method:        http.MethodDelete,
		Path:          "/api/systems/{id}",
		Summary:       "Delete a saved system",
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusForbidden, http.StatusNotFound},
	}, h.deleteSystem)
	huma.Register(api, huma.Operation{
		OperationID: "scan",
		Method:      http.MethodPost,
		Path:        "/api/scan",
		Summary:     "Scan a system's stock list",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests},
	}, h.scan)
}

// SystemsOutput lists saved systems.
type SystemsOutput struct {
	Body struct {
		Systems []rules.System `json:"systems"`
	}
}

// SystemOutput is one saved system.
type SystemOutput struct {
	Body rules.System
}

// SystemIDInput names a saved system.
type SystemIDInput struct {
	ID string `path:"id" doc:"Saved system id (a UUID)"`
}

// CreateSystemInput is a system to save.
type CreateSystemInput struct {
	Body rules.System
}

// UpdateSystemInput replaces a saved system; the body's id is ignored.
type UpdateSystemInput struct {
	ID   string `path:"id" doc:"Saved system id (a UUID)"`
	Body rules.System
}

func (h *handlers) listSystems(ctx context.Context, _ *struct{}) (*SystemsOutput, error) {
	list, err := h.systems.List(ctx)
	if err != nil {
		return nil, systemError(err)
	}
	out := &SystemsOutput{}
	out.Body.Systems = list
	return out, nil
}

func (h *handlers) getSystem(ctx context.Context, in *SystemIDInput) (*SystemOutput, error) {
	sys, err := h.systems.Get(ctx, in.ID)
	if err != nil {
		return nil, systemError(err)
	}
	return &SystemOutput{Body: sys}, nil
}

func (h *handlers) createSystem(ctx context.Context, in *CreateSystemInput) (*SystemOutput, error) {
	sys := in.Body
	sys.ID = ""
	return h.saveSystem(ctx, sys)
}

func (h *handlers) updateSystem(ctx context.Context, in *UpdateSystemInput) (*SystemOutput, error) {
	if _, err := h.systems.Get(ctx, in.ID); err != nil {
		return nil, systemError(err)
	}
	sys := in.Body
	sys.ID = in.ID
	return h.saveSystem(ctx, sys)
}

// saveSystem stores the system with its defaults filled and params
// normalized, so a reload shows exactly what runs.
func (h *handlers) saveSystem(ctx context.Context, sys rules.System) (*SystemOutput, error) {
	rules.ApplyDefaults(&sys)
	if err := rules.Validate(&sys, patterns.Default()); err != nil {
		if p := validationProblem(err, "body."); p != nil {
			return nil, p
		}
		return nil, huma.Error500InternalServerError("internal error", err)
	}
	id, err := h.systems.Save(ctx, sys)
	if err != nil {
		return nil, systemError(err)
	}
	sys.ID = id
	return &SystemOutput{Body: sys}, nil
}

func (h *handlers) deleteSystem(ctx context.Context, in *SystemIDInput) (*struct{}, error) {
	if err := h.systems.Delete(ctx, in.ID); err != nil {
		return nil, systemError(err)
	}
	return nil, nil
}

func systemError(err error) error {
	switch {
	case errors.Is(err, market.ErrNotFound):
		return huma.Error404NotFound("unknown system")
	case errors.Is(err, data.ErrReadOnly):
		return huma.Error403Forbidden("saved systems are read-only on this server")
	}
	return huma.Error500InternalServerError("internal error", err)
}

// ScanRequest scans a system's stock list (design §7.4).
type ScanRequest struct {
	System rules.System `json:"system" doc:"The system to scan; it need not be saved"`
	AsOf   market.Date  `json:"as_of,omitempty" format:"date" doc:"Scan as of this date (default: the latest benchmark date)"`
	Mode   scan.Mode    `json:"mode,omitempty" enum:"fresh,all" default:"fresh" doc:"fresh: signalled within the last 5 daily / 1 weekly bars; all: qualifying now"`
}

// ScanInput is the scan request.
type ScanInput struct {
	Body ScanRequest
}

func (h *handlers) scan(ctx context.Context, in *ScanInput) (*ScanOutput, error) {
	q := in.Body
	res, err := scan.Run(ctx, h.prov, scan.Request{System: q.System, AsOf: q.AsOf, Mode: q.Mode})
	if err != nil {
		if p := validationProblem(err, "body.system."); p != nil {
			return nil, p
		}
		return nil, mapScanError(err, q.System.Universe)
	}
	return &ScanOutput{Body: toScanResult(res)}, nil
}

// validationProblem maps a *rules.ValidationError to a 400 with one error
// per field, located under prefix; it returns nil for any other error.
func validationProblem(err error, prefix string) error {
	var ve *rules.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	details := make([]error, len(ve.Errors))
	for i, fe := range ve.Errors {
		details[i] = &huma.ErrorDetail{Message: fe.Message, Location: prefix + fe.Path}
	}
	return huma.Error400BadRequest("invalid request", details...)
}
