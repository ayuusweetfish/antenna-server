# Spirit Antenna 服务端接口

## HTTP

### 约定

载荷格式：
- 请求均为编码表单格式（Content-Type: application/x-www-form-urlencoded）。
- 响应均为 JSON 格式（Content-Type: application/json）。

错误情形：若未加说明，每个端点都可能返回以下 HTTP 状态码。这些情形下，响应的内容为纯文本，指示错误原因。
- 400 表示参数格式不正确，或取值超出范围。
- 401 表示未登录。
- 403 表示内容无权访问。
- 404 表示内容不存在。
- 500 表示服务器内部错误。

以下内容分类：
- 📙 **数据结构**：响应中以 JSON 格式组织的数据（如用户、角色档案等）。
- 🟢🔵🟣 **端点**：具体的通信地址。不同颜色的圆圈区分不同的请求方法。

### 📙 用户数据结构 User

- **id** (number) 用户 ID
- **nickname** (string) 昵称

### 🟢 注册 POST /sign-up

请求
- **nickname** (string) 昵称
- **password** (string) 密码

响应 200
- (User) 新注册的用户信息

### 🟢 登录 POST /log-in

请求
- **id** (number) 用户 ID
- **password** (string) 密码

响应 200
- (User) 登录的用户信息
- 附带一个 `Set-Cookie` 头部信息，在 `auth` 条目中存储一个登录令牌。后续请求时带上此令牌即可。

响应 401：用户名或密码错误
- 纯文本 "No such user" 或 "Incorrect password"

### 📙 角色档案数据结构 Profile

- **id** (number) 档案 ID
- **creator** (User) 创建者
- **details** (object) 角色描述（性别、取向、种族、年龄等）
  - 具体条目名称与组织方式可由客户端自行决定
- **stats** (number[8]) 八维属性值
- **traits** (string[]) 特性标签

### 🟢 创建档案 POST /profile/create

请求
- **details** (string) 角色描述（性别、取向、种族、年龄等）经过 JSON 编码的字符串
- **stats** (string) 八维属性值，以半角逗号 "," 分隔
- **traits** (string) 特性标签，以半角逗号 "," 分隔（若无，则为空字符串）

响应 200
- (Profile) 新建的档案

### 🟢 修改档案 POST /profile/{profile_id}/update

请求
- 同 **创建档案 POST /profile/create**，可省略未修改的项

响应 200
- (Profile) 修改后的档案

### 🟢 删除档案 GET /profile/{profile_id}/delete

请求
- 无参数

响应 200
- 空对象 {}

### 🔵 获取档案 GET /profile/{profile_id}

响应 200
- (Profile) 所请求的档案

### 🔵 获取玩家的档案列表 GET /profile/my

响应 200
- (Profile[]) 当前登录玩家所创建的所有角色档案

### 📙 游戏房间数据结构 Room

- **id** (string) 房间号
- **creator** (User) 房主
- **title** (string) 房间名
- **tags** (string[]) 世界观标签
- **description** (string) 世界观简介

### 🟢 创建房间 POST /room/create

请求
- **title** (string) 房间名
- **tags** (string) 世界观标签，以半角逗号 "," 分隔
- **description** (string) 世界观简介

响应 200
- (Room) 新建的房间

### 🟢 修改房间 POST /room/{room_id}/update

请求
- 同 **创建房间 POST /room/create**，可省略未修改的项

响应 200
- (Room) 修改后的房间

### 🔵 获取房间信息 GET /room/{room_id}

响应 200
- (Room) 所请求的房间

### 🟣 连接房间 GET /room/{room_id}/channel

通过 WebSocket 建立连接，会收到一条“Hello”的消息。后续通信 TODO 🚧
