# Spirit Antenna 服务端接口

## HTTP

### 约定

载荷格式：
- 请求均为编码表单格式（Content-Type: application/x-www-form-urlencoded）。
- 响应均为 JSON 格式（Content-Type: application/json）。

错误情形：在说明的情形以外，每个端点都可能返回以下 HTTP 状态码。这些情形下，响应的内容为纯文本，指示错误原因。**这些只有程序存在错误（服务端或客户端）或服务端运维失误时才会出现，正常运作时不会出现。**
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

### 🔵 关于自己 GET /me

响应 200
- (User) 当前登录玩家的用户信息

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
- **created_at** (number) 创建时刻（Unix 时间戳，以秒计）
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

上下行每条消息均为 JSON 编码的对象，均包含一个条目 **type** (string)，表示消息的类型。以下分别描述各类型消息的详情，🔻表示下行方向（服务端向客户端）、🔺表示上行方向（客户端向服务端）。列出的条目与 **type** 同级。

#### 🔻 房间状态 "room_state"
连接建立时，客户端收到一份此消息。

- **room** (Room) 房间信息
- **players** (Profile[]) 玩家（参与游戏的角色）列表
  - 组建阶段包含所有房间内的玩家。对于尚未选择角色档案的玩家，条目如下
    - **id** (null) null
    - **creator** (User) 创建者
- **my_index** (number | null) 自己在本场游戏中的座位号，对应 **players** 数组中的下标（从 0 开始）。未坐下（组建阶段）或旁观（游戏阶段）时为 null
- **phase** (string)
  - "assembly" —— 组建中，等待参与者进入、选择角色档案
  - "appointment" —— 选择起始玩家
  - "gameplay" —— 游戏进行中
- **appointment_status** (undefined | object) 游戏状态（「选择起始玩家」阶段 —— **phase**: "appointment"）
  - **holder** (number) 当前轮到的玩家座位号
  - **timer** (number) 当前轮到玩家的剩余时间，以秒计
- **gameplay_status** (undefined | object) 游戏状态（「游戏进行中」阶段 —— **phase**: "gameplay"）
  - **event** (string) 本条状态消息对应的事件
    - "none" —— 无事件，断线重连后的首条消息
      - 注："none" 是 **房间状态 "room_state"** 消息中的唯一取值。下列取值都在其他类型的消息中出现，具体见下。
    - "appointment_accept" —— 玩家接受成为起始玩家，游戏开始
    - "action_check" —— 轮到的玩家出牌并进行判定
    - "storytelling_end_next_storyteller" —— 轮到的玩家结束讲述，轮到下一位（被动方）讲述
    - "storytelling_end_new_move" —— 轮到的玩家结束讲述，开始新的回合
    - "queue" —— 有玩家加入排队
  - **is_timeout** (boolean) 事件是否是由于超时自动托管触发（可能出现于下列事件："appointment_accept"、"action_check"、"storytelling_end_next_storyteller"、"storytelling_end_new_move"）
  - **act_count** (number) 当前幕数（从 1 开始）
  - **round_count** (number) 当前轮数（从 1 开始）
  - **move_count** (number) 当前回合数（从 1 开始）
  - **relationship** (number[N, 3]) 自己与其他玩家之间的关系评价（按“激情”、“亲密”、“责任”的顺序；对应自己的一行均为 0）
  - **action_points** (number) 自己剩余的行动点数
    - 基础版 demo 阶段，为 1 表示本轮尚未发言，为 0 表示本轮已经发言、不能再举手。
  - **hand** (string[]) 自己所持有的手牌
  - **arena** (strings[]) 场上的关键词列表
  - **holder** (number) 当前轮到的玩家座位号
  - **step** (string) 当前环节
    - "selection" —— 正在选择手牌
    - "storytelling_holder" —— 主动方正在讲述
    - "storytelling_target" —— 被动方正在讲述
  - 以下带🔸的条目表示本回合进行的行动，以及判定结果。若处于「选择手牌」阶段，这些条目均为空。
  - 🔸 **action** (null | string) 当前行动的行动牌名称
  - 🔸 **keyword** (null | number) 当前行动的关键词编号（**arena** 中的下标，从 0 开始）
  - 🔸 **target** (null | number) 行动的被动方玩家座位号
  - 🔸 **holder_difficulty** (null | number) 主动方投掷出的行动难度
  - 🔸 **holder_result** (null | number) 主动方判定结果
    - 2：大成功
    - 1：成功
    - -1：失败
    - -2：大失败
  - 🔸 **target_difficulty** (null | number) 被动方投掷出的行动难度。若无被动方，则为空。
    - 此为投掷出的原始难度。根据规则，实际进行判定时使用的数值在此基础上增加 **holder_result** × -10。
  - 🔸 **target_result** (null | number) 被动方判定结果。若无被动方，则为空。
  - **timer** (number) 当前环节的剩余时间，以秒计
  - **queue** (number[]) 当前举手排队的玩家列表，靠前的玩家最先轮到

后续消息也是类似，游戏过程中指代玩家均采用座位编号，即 **players** 中的下标，从 0 开始。考虑到多语言、文本编码等因素，卡牌与关键词均使用缩略名称，名称列表 🚧。

#### 🔺 坐下 "seat"
房间组建期间，玩家发送此消息，确认所选的角色档案。

- **profile_id** (number) 所选的角色档案 ID

完成后，服务端广播一条 **组建期间房间状态变更 "assembly_update"** 消息。

#### 🔺 离座 "withdraw"
房间组建期间，玩家发送此消息，取消选择角色档案（即请求房主等待）。

- 无额外参数

完成后，服务端广播一条 **组建期间房间状态变更 "assembly_update"** 消息。

#### 🔻 组建期间房间状态变更 "assembly_update"
同 **房间状态 "room_state"**，但是只包含以下条目
- **players** (Profile[])

#### 🔺 开始游戏 "start"
房间组建期间，房主发送此消息开始游戏。

- 无额外参数

完成后，服务端广播一条 **游戏开始 "start"** 消息。

#### 🔻 开始游戏 "start"
房主确认开始游戏后，所有玩家（包括房主）收到一条此消息。房间此时进入「选择起始玩家」阶段（"appointment"）。

- **holder** (number) 轮到选择的首位玩家座位号
- **my_index** (number) 自己在本场游戏中的玩家座位号
  - 此值实为冗余信息，供参考。在最末一条 **组建期间房间状态变更 "assembly_update"** 消息的 **players** 中找到玩家自身，其下标即为 **my_index**。
- **timer** (number) 首位轮到玩家的时间限制，以秒计

#### 🔺 起始玩家指派：接受 "appointment_accept"
- 无额外参数

只有轮到自己时才有效。完成后，服务端广播一条 **起始玩家指派：接受 "appointment_accept"** 消息。

#### 🔺 起始玩家指派：跳过 "appointment_pass"
- 无额外参数

只有轮到自己时才有效。完成后，服务端广播一条 **起始玩家指派：跳过 "appointment_pass"** 消息。（对于跳过后已轮转满两轮的情况除外，见下。）

#### 🔻 起始玩家指派：接受 "appointment_accept"
表示一位玩家（也可能是自己）接受了自己作为起始玩家的指派，或者在两轮后被随机指派（在逻辑上视同随机玩家“接受”了指派）。房间此时进入「游戏进行中」阶段（"gameplay"）。

- **prev_holder** (null | number) 若为玩家自行接受，则为 null；若为两轮后随机指派，则表示最后一位跳过的玩家座位号。
- **gameplay_status** (object) 同 **房间状态 "room_state"**。其中重要的信息在此复述供参考。
  - **event** (string) 等于 "appointment_accept"
  - **is_timeout** (boolean) 事件是否是由于超时自动托管触发
  - **hand** (string[]) 自己所持有的手牌
  - **arena** (strings[]) 场上的关键词列表
  - **holder** (number) 当前轮到的座位号
    - 注：此即为接受指派/被随机指派起始的玩家座位号
  - **timer** (number) 当前环节的剩余时间，以秒计

#### 🔻 起始玩家指派：跳过 "appointment_pass"
表示一位玩家（也可能是自己）跳过自己作为起始玩家的指派。（如果玩家跳过后已轮转满两轮，则不发送此消息，而视为是随机玩家“接受”了指派，见上。）

- **prev_holder** (number) 选择跳过的玩家
- **next_holder** (number) 接下来轮到选择的玩家。等于 (**prev_holder** + 1) % N，其中 N 为玩家总数
- **is_timeout** (boolean) 事件是否是由于超时自动托管触发
- **timer** (number) 下一位轮到玩家的时间限制，以秒计

#### 🔺 打出手牌 "action"
- **hand_index** (number) 手牌的编号（**hand** 中的下标，从 0 开始）
- **arena_index** (number) 场上关键词的编号（**arena** 中的下标，从 0 开始）
- **target** (number | null | undefined) 被动方玩家的座位号。若无被动方，则可取以下任意值：① 自己的座位号、② -1、③ null、④ undefined（直接省略）

只有轮到自己时才有效。完成后，服务端广播一条 **游戏进程 "gameplay_progress"** 消息，其中 **gameplay_status.event** 值为 "action_check"，所选手牌从手牌列表中移除，玩家行动点数被扣除。

#### 🔺 讲述完成 "storytelling_end"
- 无额外参数

只有轮到自己讲述时有效。若超时，讲述环节会自动结束，不必再发送此消息。完成后，服务端广播一条 **游戏进程 "gameplay_progress"** 消息，其中 **gameplay_status.event** 值为 "storytelling_end_next_storyteller" 或 "storytelling_end_new_move"。若为后者，则补充一张手牌。

#### 🔺 举手 "queue"
- 无额外参数

其他玩家讲述期间可以举手排队。完成后，服务端广播一条 **游戏进程 "gameplay_progress"** 消息，其中 **gameplay_status.event** 值为 "queue"。

#### 🔺 评论 "comment"
- **text** (string) 发送的文字评论
- 表情 🚧

完成后，服务端广播一条 **游戏日志 "log"** 消息。

#### 🔻 游戏进程 "gameplay_progress"

- **gameplay_status** (object) 同 **房间状态 "room_state"**。

#### 🔻 游戏日志 "log"
游戏中各类事件均会产生日志。（当前均为纯文本，富文本功能 🚧）

- **log** (object[]) 多条日志
  - **id** (number) 顺序编号，可用于断线等情况下去重
  - **timestamp** (number) Unix 时间戳，以秒计
  - **content** (string) 日志文本

#### 🔻 游戏结束 "game_end"
游戏结束（最后一位玩家结束讲述）时广播此消息。

- **relationship** (number[N, 3]) 自己与其他玩家之间的关系评价（按“激情”、“亲密”、“责任”的顺序；对应自己的一行均为 0）
- **growth_points** (number) 玩家本局游戏获得的成长点数
