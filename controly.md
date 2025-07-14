# API
## 客戶端註冊
伺服器產生 uuid
```mermaid
sequenceDiagram
	client->>+server: GET /api/client/register?app=<appid>
	client<<->>server: websocket
	deactivate server
```
客戶端指定
```mermaid
sequenceDiagram
client->>+server: GET /api/client/register?app=<appid>&uuid=<uuid>
client<<->>server: websocket
deactivate server
```
## 管理端註冊
管理端所有操作都在 websocket 裡
```mermaid
sequenceDiagram
admin->>+server: GET /api/admin/ws
admin<<->>server: websocket
deactivate server
```
## 管理端連接客戶端
> in websocket
```mermaid
sequenceDiagram
admin->>server: connect=<uuid>
server->>admin: connect=<uuid> {definition of controls}
```
## 管理端執行某個動作（含修改某個值）
> in websocket
```mermaid
sequenceDiagram
admin->>server: action=<action name> data=...
server->>admin: action=<action name> data=... (response)
```
## 客戶端主動更新狀態
> in websocket
```mermaid
sequenceDiagram
client->>server: status=...
```
## 伺服器通知管理端狀態更新
> in websocket
```mermaid
sequenceDiagram
server->>admin: status=... uuid=...
```
## 客戶端向其他客戶端廣播訊息
> in websocket
```mermaid
sequenceDiagram
client->>server: broadcast=...
```
限制只向某些客戶端發送（考慮要不要限制必須是同一個 App）
```mermaid
sequenceDiagram
client->>server: broadcast=... uuid=xxx,yyy,zzz
```
## 取得 App 內的客戶端
> in websocket
```mermaid
sequenceDiagram
admin/client->>server: query="client"
server->>admin/client: clients=[xxx, yyy, ...]
```
## App
App 是指一套關於如何控制客戶端的設定，使用密碼可以修改 App。有一套基本的 CURD API 可以建立、修改、查詢、刪除
POST /api/app
PUT /api/app/\<app>
GET /api/app
GET /api/app/\<app>
DELETE /api/app/\<app>

```typescript
type App = {
	name: string
	password: string // not exported
	controls: Control[]
}
```
# Control Definition
```typescript
type Control = {
	id: string
	type: 'button'
	value: string
} | {
	id: string
	type: 'number'
	min?: number
	max?: number
	step?: number
} | {
	id: string
	type: 'string'
}
```

# 簡介
須事先定義 app，app 是一堆 control 的集合，例如有按鈕、文字框、數字框等等。
client 連線到 server 時可以主動提供預先分配好的 uuid，或是由伺服器指定，並且告訴伺服器他所屬於的 app，同時建立 websocket 連線。
admin 透過手動輸入、QRCode 之類的機制取得 uuid 後，向 server 取得 app 設定，然後根據 app 設定顯示相應的控制 UI。
admin 可以向 server 送出要控制哪個 client 的哪個 control，client 也可以主動向 server 更新某個狀態，然後由 server 向對這個 client 有連線的 admin 發送狀態更新。
# 例子
一個倒數計時網頁，client 只有一個大大的時間，剩下的控制界面都在 admin。
## client
```mermaid
flowchart TD

A(["開啟 Client 頁面"]) --> B{"是否自備 UUID"}

B -- 是 --> C@{ label: "<span style=\"color:\" color=\"\">POST /api/client/register</span><br style=\"box-sizing:\" color=\"\">{ app: app name, uuid: uuid }" }

B -- 否 --> D@{ label: "<span style=\"padding-left:\"><span style=\"color:\" color=\"\">POST /api/client/register</span><br style=\"box-sizing:\" color=\"\">{ app: app name }</span>" }

C --> n1["UUID 是否可用"]

n1 -- 否 --> n2(["顯示錯誤訊息"])

n1 -- 是 --> n3>"GET /api/client/ws?uuid=uuid"]

D --> n3

n3 --> n4(["開啟 Websocket 連線"])

  

C@{ shape: odd}

D@{ shape: odd}

n1@{ shape: decision}
```
client 建立 websocket 連線後註冊相關 control
```
client.on('start', timer.start)
client.on('pause', timer.pause)
client.on('restart', timer.restart)
client.on('set', timer.set) // timer.set = (number) => void
```
接著 client 會自動在收到相對應的 websocket 請求時執行函數。另外當 `timer.start` 被執行時，會向 server 更新 `timer.time` `client.updateStatue({ time: timer.time, counting: true })`

## admin
admin 打開網頁後，建立一個 websocket 連線。然後輸入一個或多個 client 的 uuid，伺服器回傳 client 的 app 設定，接著顯示對應的控制畫面。並且在接收到 status 的通知時更新畫面。
```mermaid
flowchart TD

subgraph s1["Websocket"]

n5(["輸入 client uuid"])

n6>"GET /api/client/{uuid}"]

n7["根據回應更新 Control"]

n8(["收到更新 type=status"])

n9(["當觸發 name={name} 的 control<br>（可能有 value={value}）"])

n10>"POST /api/client/{uuid}<br>{ name: name, value: value }"]

end

n1(["開啟 Admin 頁面"]) --> n2>"GET /api/admin/ws"]

n2 --> n3["開啟 Websocket 連線"]

n3 --> s1

n5 --> n6

n6 --> n7

n9 --> n10

n10 --> n7

n8 --> n7

n3@{ shape: rect}
```
# struct
```mermaid
classDiagram

direction TB

class App {

name

password

controls

}

  

class ButtonControl {

name

}

  

class TextControl {

name

regex

}

  

class NumberControl {

name

min

max

int

}

  

class Status {

any

}

  

class Client {

app

admins

status

Emit(name string, data any)

Update(status Status)

}

  

class Admin {

clients

Connect(clientID)

Request(name string, value any)

}

  

class Node {

websocket

id

Send(data)

On(event string, data any)

}

  

<<Control>> ButtonControl

<<Control>> TextControl

<<Control>> NumberControl

  

Client "n" --|> "1" App

Client --|> Status

App --|> ButtonControl

App --|> TextControl

App --|> NumberControl

Node --|> Admin : extends

Node --|> Client : extends

Admin <|--|> Client
```
```go
type Admin struct {
	id string // uuid
	clients []Client
}

type Client struct {
	id string // uuid
	app *App
}

type App struct {
	name string
	password string
	controls []Control
}

type Control struct {
	name string
	type string
}
```
