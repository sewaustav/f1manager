package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"testing"

	"f1/internal/models"
	"f1/internal/web/dto"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeData implements the Data interface with seeded in-memory values.
type fakeData struct {
	engines []models.Engine
	budget  int
	tokens  int
}

func (f *fakeData) GetPilotsService(context.Context, int64) ([]models.Pilot, error) { return nil, nil }
func (f *fakeData) GetTeamsService(context.Context, int64) ([]models.Team, error) {
	return nil, nil
}
func (f *fakeData) GetPrincipalsService(context.Context) ([]models.TeamPrincipal, error) {
	return nil, nil
}
func (f *fakeData) GetTrackInfoService(context.Context, string) ([]models.Track, error) {
	return nil, nil
}
func (f *fakeData) GetMyTeamService(context.Context, int64) (models.MyTeam, error) {
	return models.MyTeam{}, nil
}
func (f *fakeData) GetPlayersService(context.Context, int64) ([]models.Player, error) {
	return nil, nil
}
func (f *fakeData) GetPlayersTeamsService(context.Context, int64) ([]models.MyTeam, error) {
	return nil, nil
}
func (f *fakeData) GetEnginesService(context.Context) ([]models.Engine, error) {
	return f.engines, nil
}
func (f *fakeData) GetBudgetService(context.Context, int64, int64) (int, int, error) {
	return f.budget, f.tokens, nil
}

// fakeUser implements the User interface.
type fakeUser struct {
	group *int64
}

func (f *fakeUser) GetUserGroup(context.Context, int64) (*int64, error)   { return f.group, nil }
func (f *fakeUser) RegisterGroup(context.Context, int64, dto.Group) error { return nil }
func (f *fakeUser) JoinGroup(context.Context, int64, dto.Group) error     { return nil }
func (f *fakeUser) ResetGroup(context.Context, int64) error               { return nil }

func setupEnginesBudget(t *testing.T, d Data, u User) (*gin.Engine, *rsa.PrivateKey) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	h := NewHttpHandler(nil, nil, d, u, nil, nil, nil, nil, nil, nil)

	r := gin.New()
	v1 := r.Group("/api/v1")
	middleware := jwtmw.New(&key.PublicKey, "f1", "f1")
	game := v1.Group("")
	game.Use(middleware.Handler())
	game.GET("/engines", h.GetEngines)
	game.GET("/budget", h.GetBudget)

	return r, key
}

func TestGetEngines(t *testing.T) {
	engines := []models.Engine{
		{ID: 1, Engine: models.ICEName(1), Price: 100, BaseLevel: 5},
	}
	groupID := int64(7)
	data := &fakeData{engines: engines, budget: 50, tokens: 3}
	r, key := setupEnginesBudget(t, data, &fakeUser{group: &groupID})

	w := doReq(r, http.MethodGet, "/api/v1/engines", "", token(t, key, "1"))
	require.Equal(t, 200, w.Code)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.NotEmpty(t, got)
	require.Contains(t, got[0], "ID")
}

func TestGetBudget(t *testing.T) {
	groupID := int64(7)
	data := &fakeData{engines: nil, budget: 42, tokens: 9}
	r, key := setupEnginesBudget(t, data, &fakeUser{group: &groupID})

	w := doReq(r, http.MethodGet, "/api/v1/budget", "", token(t, key, "1"))
	require.Equal(t, 200, w.Code)

	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Contains(t, got, "budget")
	require.Contains(t, got, "tokens")
}

func TestGetBudgetNoGroup(t *testing.T) {
	data := &fakeData{}
	r, key := setupEnginesBudget(t, data, &fakeUser{group: nil})

	w := doReq(r, http.MethodGet, "/api/v1/budget", "", token(t, key, "1"))
	require.Equal(t, 400, w.Code)
}
