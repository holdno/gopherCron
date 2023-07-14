package app

import (
	"context"
	"time"

	"github.com/holdno/gopherCron/config"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type OIDCService struct {
	app      *app
	cfg      config.OIDC
	provider *oidc.Provider
	state    string
	oauth2   oauth2.Config
}

func NewOIDCService(app *app, cfg config.OIDC) (*OIDCService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	srv := &OIDCService{
		app:      app,
		cfg:      cfg,
		provider: provider,
		state:    "gophercron",
	}

	srv.oauth2 = oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes,
	}
	return srv, nil
}

func (s *OIDCService) AuthURL() string {
	return s.oauth2.AuthCodeURL(s.state)
}

func (s *OIDCService) Login(ctx context.Context, code, state string) (*oauth2.Token, error) {
	tokens, err := s.oauth2.Exchange(ctx, code)
	if err != nil {
		wlog.Error("failed to get tokens from oidc service", zap.Error(err), zap.String("code", code))
		return nil, err
	}
	return tokens, nil
}

func (s *OIDCService) GetUserInfo(ctx context.Context, accessToken string) (*oidc.UserInfo, error) {
	info, err := s.provider.UserInfo(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	}))
	if err != nil {
		wlog.Error("failed to get userinfo from oidc service", zap.Error(err))
		return nil, err
	}
	return info, nil
}

func (s *OIDCService) UserNameKey() string {
	return s.cfg.UserNameKey
}
