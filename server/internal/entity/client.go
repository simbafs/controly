package entity

type Client struct {
	Node
	app    *App
	admin  *Admin
	status map[string]any
}
