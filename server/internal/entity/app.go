package entity

import (
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

const BcryptCost = 10

type App struct {
	name     string
	password []byte // bcrypt hashed password
	controls []Control
}

func NewApp(name string, password string) (*App, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return nil, err
	}
	return &App{
		name:     name,
		password: hashed,
		controls: make([]Control, 0),
	}, nil
}

func (a *App) Name() string {
	return a.name
}

func (a *App) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword(a.password, []byte(password))
	if err != nil {
		slog.Error("failed to verify app password", "app", a.name, "error", err)
	}
	return err == nil
}

func (a *App) Controls() []Control {
	return a.controls
}

func (a *App) AppendControl(control ...Control) {
	a.controls = append(a.controls, control...)
}
