# Spirit Antenna 服务端接口

## HTTP

### 约定

载荷格式：
- 请求均为编码表单格式（Content-Type: application/x-www-form-urlencoded）。
- 响应均为 JSON 格式（Content-Type: application/json）。

错误情形：若未加说明，每个端点都可能返回以下 HTTP 状态码。这些情形下，响应的内容为纯文本，指示错误原因。**这些错误表明程序存在错误（服务端或客户端）或服务端运维失误。**
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

通过 WebSocket 建立连接。

房间不存在或已关闭时会返回 404 状态码并拒绝连接。（游戏尚未开始时，房主退出 3 分钟后房间关闭，房间内所有连接断开。房主重新进入后即再次开启。）

上下行每条消息均为 JSON 编码，均包含一个条目 **type** (string)，表示消息的类型。以下分别描述各类型消息的详情，⬇️表示下行方向（服务端向客户端）、⬆️表示上行方向（客户端向服务端）。

#### ⬇️ 房间状态 "room_state"
连接建立时，客户端收到一份此消息。

- **room** (Room) 房间信息
- **players** (Profile[]) 玩家（参与游戏的角色）列表
  - 组建阶段包含所有房间内的玩家。对于尚未选择角色档案的玩家，条目如下
    - **id** (null) null
    - **creator** (User) 创建者
- **phase** (string)
  - "assembly" —— 组建中，等待参与者进入、选择角色档案
  - "appointment" —— 选择起始玩家
  - "gameplay" —— 游戏进行中
- **appointment_status** (undefined | object) 游戏状态（「选择起始玩家」阶段 —— **phase**: "apoointment"）
  - **holder** (number) 当前轮到的玩家编号（从 0 开始）
  - **timer** (number) 当前玩家的剩余时间，以秒计
- **gameplay_status** (undefined | object) 游戏状态（「游戏进行中」阶段 —— **phase**: "gameplay"）
  - **act_count** (number) 当前幕数（从 1 开始）
  - **turn_count** (number) 当前轮数（从 1 开始）
  - **move_count** (number) 当前回合数（从 1 开始）
  - **relationship** (number[N, 3]) 与其他玩家之间的关系评价
  - **hand** (string[]) 当前玩家所持有的手牌
  - **arena** (strings[]) 场上的关键词列表
  - **holder** (number) 当前轮到的玩家编号（从 0 开始）
  - **step** (string) 当前环节
    - "selection" —— 正在选择手牌；此时下述 **action**、**keyword** 与 **target** 三项均为 null
    - "storytelling_holder" —— 发起方正在讲述
    - "storytelling_target" —— 被动方正在讲述
  - **action** (null | number) 当前选择的行动牌
  - **keyword** (null | number) 当前选择的关键词
  - **target** (null | number) 行动的被动方玩家编号（从 0 开始）
  - **timer** (number) 当前环节的剩余时间，以秒计
  - **queue** (number[]) 当前举手排队的玩家列表，靠前的玩家最先轮到

#### ⬆️ 坐下 "seat"
房间组建期间，玩家发送此消息，确认所选的角色档案。

- **profile_id** (number) 所选的角色档案 ID

完成后，服务端广播一条 **组建期间房间状态更新 "assembly_update"** 消息。

#### ⬆️ 离座 "withdraw"
房间组建期间，玩家发送此消息，取消选择角色档案（即请求房主等待）。

- 无额外参数

完成后，服务端广播一条 **组建期间房间状态更新 "assembly_update"** 消息。

#### ⬇️ 组建期间房间状态更新 "assembly_update"
同 **房间状态 "room_state"**，但是只包含以下条目
- **players**

#### ⬆️ 开始游戏 "start"
房间组建期间，房主发送此消息开始游戏。

- 无额外参数

完成后，服务端广播一条 **游戏开始 "start"** 消息，后接一条 **房间状态 "room_state"** 消息。

#### ⬇️ 开始游戏 "start"
房主确认开始游戏后，所有玩家（包括房主）收到一条此消息。

- 无额外参数
