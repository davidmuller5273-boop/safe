# SafeW Bot API 完整文档

> 来源：https://docs.safew.org/bot-api/
> 本文件按网站左侧目录顺序合并，共包含全部中文 Bot API 文档。

## 目录

- **Bot API**
  - [概览](#page-index)
- **基础**
  - [getMe](#page-getme)
  - [getUpdates](#page-getupdates)
  - [消息格式化](#page-formatting)
- **消息**
  - [sendMessage](#page-sendmessage)
  - [sendMessageDraft](#page-sendmessagedraft)
  - [sendPhoto](#page-sendphoto)
  - [sendVideo](#page-sendvideo)
  - [sendVoice](#page-sendvoice)
  - [sendAudio](#page-sendaudio)
  - [sendDocument](#page-senddocument)
  - [sendMediaGroup](#page-sendmediagroup)
  - [sendDice](#page-senddice)
  - [editMessageText](#page-editmessagetext)
  - [editMessageReplyMarkup](#page-editmessagereplymarkup)
  - [deleteMessage](#page-deletemessage)
  - [deleteMessages](#page-deletemessages)
  - [copyMessage](#page-copymessage)
  - [pinChatMessage](#page-pinchatmessage)
  - [unpinChatMessage](#page-unpinchatmessage)
- **Webhook**
  - [setWebhook](#page-setwebhook)
  - [deleteWebhook](#page-deletewebhook)
  - [getWebhookInfo](#page-getwebhookinfo)
- **聊天管理**
  - [getChat](#page-getchat)
  - [getChatMemberCount](#page-getchatmembercount)
  - [getChatMember](#page-getchatmember)
  - [getChatAdministrators](#page-getchatadministrators)
  - [setChatTitle](#page-setchattitle)
  - [leaveChat](#page-leavechat)
  - [setChatPermissions](#page-setchatpermissions)
- **成员管理**
  - [banChatMember](#page-banchatmember)
  - [unbanChatMember](#page-unbanchatmember)
  - [banChatSenderChat](#page-banchatsenderchat)
  - [unbanChatSenderChat](#page-unbanchatsenderchat)
  - [promoteChatMember](#page-promotechatmember)
  - [restrictChatMember](#page-restrictchatmember)
  - [setMyDefaultAdministratorRights](#page-setmydefaultadministratorrights)
  - [setChatAdministratorCustomTitle](#page-setchatadministratorcustomtitle)
- **邀请链接**
  - [createChatInviteLink](#page-createchatinvitelink)
  - [editChatInviteLink](#page-editchatinvitelink)
  - [revokeChatInviteLink](#page-revokechatinvitelink)
  - [exportChatInviteLink](#page-exportchatinvitelink)
  - [approveChatJoinRequest](#page-approvechatjoinrequest)
  - [declineChatJoinRequest](#page-declinechatjoinrequest)
- **其他**
  - [answerCallbackQuery](#page-answercallbackquery)
  - [answerInlineQuery](#page-answerinlinequery)
  - [setMyCommands](#page-setmycommands)
  - [getMyCommands](#page-getmycommands)
  - [getFile](#page-getfile)
  - [getUserProfilePhotos](#page-getuserprofilephotos)
- **小游戏**
  - [概览](#page-games)
  - [sendGame](#page-sendgame)
  - [setGameScore](#page-setgamescore)
  - [getGameHighScores](#page-getgamehighscores)

---

## Bot API

<a id="page-index"></a>

### Bot API 概览

SafeW Bot API 的核心是一套通过 HTTPS 请求进行交互、以 JSON 格式返回响应的软件服务。

Bot 本质上是一段程序、脚本或服务，通过 HTTPS 请求查询 API 并等待响应。你可以发起多种类型的请求，也可以使用和接收多种不同的对象。

#### 获取 Bot Token

Token 是一个字符串，用于在 Bot API 上验证你的 Bot（而非你的账号）身份。每个 Bot 都有唯一的 Token，可以随时通过 @BotFather 撤销。

获取 Token 非常简单：联系 @BotFather，发送 `/newbot` 命令并按照步骤操作即可获得新 Token。你可以在[快速开始](https://docs.safew.org/guide/quickstart)中找到详细指引。

Token 格式如下：

```
4839574812:AAFD39kkdpWt3ywyRZergyOLMaJhac60qc
```

#### 发起请求

所有请求通过 URL 中的 Token 进行认证，格式为：

```
https://api.safew.bot/bot<YOUR_BOT_TOKEN>/methodName
```

由于浏览器本身就能发送 HTTPS 请求，你可以用它快速体验 API。获取 Token 后，尝试在浏览器中粘贴以下地址：

```
https://api.safew.bot/bot<YOUR_BOT_TOKEN>/getMe
```

理论上，你可以通过浏览器或 cURL 等工具发送这样的基本请求来与 API 交互。虽然对于上面这种简单请求来说没问题，但对于更大型的应用来说并不实用，也难以扩展。

请求数据格式为 JSON，文件上传使用 `multipart/form-data`。

#### 标准响应格式

##### 成功响应

```
{
  "ok": true,
  "result": {
    ...
  }
}
```

##### 错误响应

```
{
  "ok": false,
  "error_code": 400,
  "description": "Bad Request: ...",
  "trace_id": "abc123"
}
```

#### 通用错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权执行该操作 |
| 404 | 方法不存在或资源未找到 |
| 429 | 请求过于频繁，触发限流 |
| 500 | 服务器内部错误 |

#### 可用方法

##### 基础方法

| 方法 | 描述 | HTTP 方法 |
| --- | --- | --- |
| [getMe](#page-getme) | 获取 Bot 基本信息 | GET / POST |
| [getUpdates](#page-getupdates) | 长轮询获取更新 | GET / POST |

##### 发送消息

| 方法 | 描述 | HTTP 方法 |
| --- | --- | --- |
| [sendMessage](#page-sendmessage) | 发送文本消息 | POST |
| [sendMessageDraft](#page-sendmessagedraft) | 向用户私聊输入框推送草稿（SafeW 扩展） | POST |
| [sendPhoto](#page-sendphoto) | 发送图片 | POST |
| [sendVideo](#page-sendvideo) | 发送视频 | POST |
| [sendVoice](#page-sendvoice) | 发送语音 | POST |
| [sendAudio](#page-sendaudio) | 发送音频 | POST |
| [sendDocument](#page-senddocument) | 发送文档 | POST |
| [sendMediaGroup](#page-sendmediagroup) | 发送媒体组 | POST |
| [sendDice](#page-senddice) | 发送随机动画表情 | POST |

##### 编辑消息

| 方法 | 描述 | HTTP 方法 |
| --- | --- | --- |
| [editMessageText](#page-editmessagetext) | 编辑消息文本 | POST |
| [editMessageReplyMarkup](#page-editmessagereplymarkup) | 编辑消息回复标记 | POST |

##### 小游戏

| 方法 | 描述 | HTTP 方法 |
| --- | --- | --- |
| [小游戏概览](#page-games) | HTML5 游戏接入说明与流程 | — |
| [sendGame](#page-sendgame) | 发送游戏卡片 | POST |
| [setGameScore](#page-setgamescore) | 提交/更新用户分数 | POST |
| [getGameHighScores](#page-getgamehighscores) | 获取游戏高分榜 | POST |

##### 内联模式

| 方法 | 描述 | HTTP 方法 |
| --- | --- | --- |
| [answerInlineQuery](#page-answerinlinequery) | 回复内联查询 | POST |

#### 文件上传说明

以下方法支持文件上传，需使用 `multipart/form-data` 编码：

- `sendPhoto` - 图片文件，最大 10MB
- `sendVideo` - 视频文件
- `sendVoice` - 语音文件
- `sendAudio` - 音频文件
- `sendDocument` - 文档文件，最大 100MB
- `sendMediaGroup` - 媒体组，最多 10 个文件

除了直接上传文件外，也可以传入已上传文件的 `file_id` 来复用已有文件。

[返回目录](#目录) · [原网页](https://docs.safew.org/bot-api/)

---

## 基础

<a id="page-getme"></a>

### getMe

获取 Bot 的基本信息。可用于测试 Bot Token 是否有效。

#### 请求

`GET /:token/getMe`

`POST /:token/getMe`

本方法同时支持 GET 和 POST 请求。

#### 参数

无需任何参数。

#### 响应

返回 Bot 的基本信息对象。

```
{
  "ok": true,
  "result": {
    "id": 123456789,
    "is_bot": true,
    "username": "my_bot",
    "can_join_groups": true,
    "can_read_all_group_messages": true,
    "supports_inline_queries": true
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| id | Integer | Bot 的唯一标识符 |
| is\_bot | Boolean | 是否为 Bot，始终为 `true` |
| username | String | Bot 的用户名 |
| can\_join\_groups | Boolean | Bot 是否可以被邀请加入群组，始终为 `true` |
| can\_read\_all\_group\_messages | Boolean | Bot 是否可以读取所有群组消息，始终为 `true` |
| supports\_inline\_queries | Boolean | Bot 是否支持内联查询，始终为 `true` |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 401 | Token 无效或已过期 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X GET "https://api.safew.bot/<token>/getMe"
```

##### 响应

```
{
  "ok": true,
  "result": {
    "id": 123456789,
    "is_bot": true,
    "username": "my_bot",
    "can_join_groups": true,
    "can_read_all_group_messages": true,
    "supports_inline_queries": true
  }
}
```

[返回目录](#目录) · [原网页](#page-getme)

---

<a id="page-getupdates"></a>

### getUpdates

通过长轮询方式获取 Bot 的最新更新。返回 Update 对象数组。

#### 请求

`GET /:token/getUpdates`

`POST /:token/getUpdates`

本方法同时支持 GET 和 POST 请求。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| offset | Integer | 否 | 更新偏移量。传入已处理的最新 update\_id + 1，服务端将只返回该偏移量之后的更新 |
| limit | Integer | 否 | 限制返回的更新数量，取值范围 1-100，默认 100 |
| timeout | Integer | 否 | 长轮询超时时间（秒）。设为 0 表示立即返回，建议设为 30 左右以减少请求频率 |
| allowed\_updates | String[] | 否 | 允许接收的更新类型列表，例如 `["message", "callback_query"]` |

#### 响应

返回 Update 对象数组。如果没有新的更新，返回空数组。

```
{
  "ok": true,
  "result": [
    {
      "update_id": 100000001,
      "message": {
        "message_id": 1,
        "from": {
          "id": 987654321,
          "is_bot": false,
          "first_name": "User",
          "username": "user123"
        },
        "chat": {
          "id": 987654321,
          "first_name": "User",
          "username": "user123",
          "type": "private"
        },
        "date": 1700000000,
        "text": "Hello, Bot!"
      }
    }
  ]
}
```

##### Update 对象字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| update\_id | Integer | 更新的唯一标识符，递增 |
| message | Message | 可选，新收到的消息 |
| edited\_message | Message | 可选，已编辑的消息 |
| channel\_post | Message | 可选，频道新消息 |
| edited\_channel\_post | Message | 可选，频道已编辑的消息 |
| business\_connection | BusinessConnection | 可选，Bot 与商业帐户之间的连接状态变更 |
| business\_message | Message | 可选，通过商业帐户收到的新消息 |
| edited\_business\_message | Message | 可选，通过商业帐户编辑的消息 |
| deleted\_business\_messages | DeletedBusinessMessages | 可选，通过商业帐户删除的消息 |
| message\_reaction | MessageReactionUpdated | 可选，消息回应变更 |
| message\_reaction\_count | MessageReactionCountUpdated | 可选，匿名消息回应计数变更 |
| inline\_query | InlineQuery | 可选，收到的内联查询 |
| chosen\_inline\_result | ChosenInlineResult | 可选，用户选择的内联查询结果 |
| callback\_query | CallbackQuery | 可选，回调查询 |
| shipping\_query | ShippingQuery | 可选，收到的配送查询 |
| pre\_checkout\_query | PreCheckoutQuery | 可选，收到的预结账查询 |
| purchased\_paid\_media | PurchasedPaidMedia | 可选，付费媒体购买信息 |
| poll | Poll | 可选，投票状态变更 |
| poll\_answer | PollAnswer | 可选，用户投票答案变更 |
| my\_chat\_member | ChatMemberUpdated | 可选，Bot 自身的聊天成员状态变更 |
| chat\_member | ChatMemberUpdated | 可选，其他聊天成员状态变更 |
| chat\_join\_request | ChatJoinRequest | 可选，加入聊天请求 |
| chat\_boost | ChatBoostUpdated | 可选，聊天 Boost 变更 |
| removed\_chat\_boost | ChatBoostRemoved | 可选，聊天 Boost 被移除 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 offset 格式不正确 |
| 401 | Token 无效或已过期 |
| 409 | 存在冲突，可能是 Webhook 已设置，无法同时使用长轮询 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/getUpdates" \
  -H "Content-Type: application/json" \
  -d '{
    "offset": 100000002,
    "limit": 10,
    "timeout": 30
  }'
```

##### 无更新时的响应

```
{
  "ok": true,
  "result": []
}
```

[返回目录](#目录) · [原网页](#page-getupdates)

---

<a id="page-formatting"></a>

### 消息格式化

Bot API 支持带格式的消息文本。发送时通过 `parse_mode` 参数指定解析模式，服务端会把标记解析为格式实体（entities）后下发给客户端。

- 可选值：`MarkdownV2`、`Markdown`、`HTML`，不区分大小写。
- 在 SafeW 中，`Markdown` 与 `MarkdownV2` 由同一解析器处理，行为完全一致。
- 提供 `parse_mode` 时，请求中的 `entities` / `caption_entities` 参数会被忽略；不提供 `parse_mode` 时才使用手动传入的实体列表。

支持 `parse_mode` 的位置：

| 方法 | 作用字段 |
| --- | --- |
| [sendMessage](#page-sendmessage)、[sendMessageDraft](#page-sendmessagedraft)、[editMessageText](#page-editmessagetext) | `text` |
| [sendPhoto](#page-sendphoto)、[sendVideo](#page-sendvideo)、[sendVoice](#page-sendvoice)、[sendAudio](#page-sendaudio)、[sendDocument](#page-senddocument)、[sendMediaGroup](#page-sendmediagroup) | `caption` |

#### MarkdownV2 风格

语法总览：

````
*粗体* 或 **粗体**
_斜体_
__下划线__
~删除线~
||剧透文本||
[内联链接](https://www.example.com)
[提及用户](safew://user?id=123456789)
![😀](safew://emoji?id=5368324170671202286)
`行内代码`
```代码块```
```python
多行代码块
（第一行反引号后的首个单词作为语言标识）
```
> 引用第一行
> 引用第二行
**> 可折叠引用，以 || 结束||
````

##### 文本样式

| 样式 | 写法 | 说明 |
| --- | --- | --- |
| 粗体 | `*文本*` 或 `**文本**` | 单星号和双星号等价，均为粗体 |
| 斜体 | `_文本_` |  |
| 下划线 | `__文本__` |  |
| 删除线 | `~文本~` | 单个波浪线 |
| 剧透 | `||文本||` | 内容默认打码，点击后显示 |

样式可以嵌套（如 `*粗体 _粗斜体_*`），也可以在引用内使用。注意 `***文本***` 不会产生「粗斜体」——三个星号会被解析为 `**` 加 `*`，两者都是粗体标记，互相抵消后不产生任何格式。

##### 代码

| 写法 | 效果 |
| --- | --- |
| `` `代码` `` | 行内代码（等宽显示） |
| ```` ```代码``` ````（同一行） | 代码块，无语言标识 |
| ```` ``` ```` 起止的多行块 | 代码块；开头 ```` ``` ```` 之后同一行的第一个单词作为语言标识（如 ```` ```python ````） |

代码内部不解析其他格式标记，但反斜杠转义依然生效（例如在行内代码中用 `` \` `` 可输出反引号本身）。

##### 链接、提及与自定义表情

| 写法 | 效果 |
| --- | --- |
| `[显示文本](https://example.com)` | 内联链接。URL 仅支持 `http://`、`https://`、`safew://` 协议，其他协议的链接会被丢弃、只保留文本 |
| `[显示文本](safew://user?id=<用户ID>)` | 提及用户（无需知道用户名，点击跳转到该用户） |
| `![占位文本](safew://emoji?id=<表情文档ID>)` | 自定义表情；URL 必须形如 `safew://emoji?id=<数字>`，占位文本在表情不可用时显示 |

##### 引用

- 行首的 `>`（后可跟一个空格）开始一段引用，连续以 `>` 开头的行合并为同一段引用。
- 行首的 `**>` 是可折叠引用语法，以 `||` 结束。**当前版本会接受该语法，但客户端按普通引用显示**（暂不支持折叠）。
- 引用内可以继续使用文本样式、行内代码等标记；可折叠引用内不要使用剧透标记（`||` 会被视为引用结束符）。

##### 转义

在标记字符前加反斜杠 `\` 可以按原样显示该字符。支持转义的字符集为：

```
* _ ~ | [ ] ( ) ` > \
```

与 Telegram 官方 MarkdownV2 的差异：

- 官方要求转义 `. ! # + - = { }` 等所有保留字符，SafeW **不需要**——这些字符本身没有特殊含义；并且对上述字符集之外的字符使用 `\` 时，反斜杠会**原样保留**（如 `\.` 会显示为 `\.`）。
- 官方对未闭合的标记返回 400 错误；SafeW **不报错**，未闭合的标记会一直作用到文本末尾。建议始终成对书写标记。

#### Markdown 风格

`parse_mode=Markdown` 与 `MarkdownV2` 完全等价（同一解析器），不存在 Telegram 官方的旧版 Markdown 差异。新接入建议直接使用 `MarkdownV2`。

#### HTML 风格

```
<b>粗体</b>、<strong>粗体</strong>
<i>斜体</i>、<em>斜体</em>
<u>下划线</u>、<ins>下划线</ins>
<s>删除线</s>、<strike>删除线</strike>、<del>删除线</del>
<span class="safew-spoiler">剧透</span>、<safew-spoiler>剧透</safew-spoiler>
<a href="https://www.example.com">内联链接</a>
<code>行内代码</code>
<pre>代码块</pre>
<pre class="language-python">带语言标识的代码块</pre>
<blockquote>引用</blockquote>
<safew-emoji emoji-id="5368324170671202286">😀</safew-emoji>
```

注意事项：

- 仅支持上表中的标签，**不支持的标签会连同尖括号原样显示**（不会报错）。
- 正文中的 `<`、`>`、`&` 需分别写成 `&lt;`、`&gt;`、`&amp;`。
- 代码块的语言标识写在 `<pre>` 标签的 `class="language-xxx"` 上；`<pre><code>...</code></pre>` 嵌套时只生成一个代码块实体。
- `<a>` 的 `href` 为任意非空值时生成链接。
- `<blockquote>` 的 `expandable` 属性当前不生效，按普通引用显示。
- `<safew-emoji>` 的 `emoji-id` 为自定义表情的文档 ID，标签内的文本作为表情不可用时的占位。

#### 示例

##### MarkdownV2

```
curl -X POST "https://api.safew.bot/<token>/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "text": "*订单已发货* 🎉\n单号：`SF1234567890`\n>物流信息请点击 [此处](https://example.com/track) 查询",
    "parse_mode": "MarkdownV2"
  }'
```

##### HTML

```
curl -X POST "https://api.safew.bot/<token>/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "text": "<b>订单已发货</b> 🎉\n单号：<code>SF1234567890</code>\n<blockquote>物流信息请点击 <a href=\"https://example.com/track\">此处</a> 查询</blockquote>",
    "parse_mode": "HTML"
  }'
```

[返回目录](#目录) · [原网页](#page-formatting)

---

## 消息

<a id="page-sendmessage"></a>

### sendMessage

发送文本消息到指定聊天。

#### 请求

`POST /:token/sendMessage`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| text | String | 是 | 消息文本内容 |
| business\_connection\_id | String | 否 | 商业连接的唯一标识符，用于代表商业帐户发送消息 |
| message\_thread\_id | Integer | 否 | 目标消息线程（话题）的唯一标识符，仅用于超级群和频道 |
| parse\_mode | String | 否 | 消息文本解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| entities | MessageEntity[] | 否 | 消息文本中的特殊实体列表，可替代 `parse_mode` 使用 |
| link\_preview\_options | Object | 否 | 链接预览生成选项 |
| disable\_notification | Boolean | 否 | 静默发送消息，用户将收到无声通知 |
| protect\_content | Boolean | 否 | 保护消息内容不被转发和保存 |
| allow\_paid\_broadcast | Boolean | 否 | 允许向频道聊天发送付费广播消息 |
| message\_effect\_id | String | 否 | 消息特效的唯一标识符 |
| reply\_parameters | Object | 否 | 回复参数，描述要回复的消息 |
| reply\_markup | Object | 否 | 自定义键盘或内联键盘标记，JSON 序列化对象 |
| reply\_to\_message\_id | Integer | 否 | 要回复的消息 ID，设置后该消息将作为引用回复发送 |

#### 响应

返回发送成功的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 100,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "text": "Hello, World!"
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 text 或 chat\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 被该用户封禁或无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "text": "Hello, World!",
    "parse_mode": "HTML"
  }'
```

##### 带内联键盘的消息

```
curl -X POST "https://api.safew.bot/<token>/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "text": "请选择一个选项：",
    "reply_markup": {
      "inline_keyboard": [
        [
          {"text": "选项 A", "callback_data": "option_a"},
          {"text": "选项 B", "callback_data": "option_b"}
        ]
      ]
    }
  }'
```

##### 回复消息

```
curl -X POST "https://api.safew.bot/<token>/sendMessage" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "text": "这是一条回复",
    "reply_to_message_id": 99
  }'
```

[返回目录](#目录) · [原网页](#page-sendmessage)

---

<a id="page-sendmessagedraft"></a>

### sendMessageDraft

向指定用户的私聊输入框推送一段草稿文本。这是 SafeW 的扩展方法，Telegram Bot API 中不存在。

与 `sendMessage` 不同，本方法**不会创建真实消息**：草稿以实时更新的形式下发，客户端收到后在该用户与 Bot 私聊的输入框中预填这段文本，由用户决定是否发送。草稿不落库、不产生 `message_id`。

#### 请求

`POST /:token/sendMessageDraft`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标用户的数字 ID（或数字字符串）。仅支持私聊用户，不支持群组、频道或 `@username` 形式 |
| draft\_id | Integer | 是 | 草稿的唯一标识符（随机 ID），必须非 0。客户端用它对同一草稿的多次推送去重 |
| text | String | 是 | 草稿文本内容，1-4096 个字符 |
| parse\_mode | String | 否 | 文本解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| entities | MessageEntity[] | 否 | 草稿文本中的特殊实体列表，可替代 `parse_mode` 使用。同时提供时以 `parse_mode` 为准 |

#### 使用限制

- 仅限私聊：`chat_id` 必须是用户 ID，不能是 Bot、群组或频道。
- 目标用户必须与 Bot 存在会话且给 Bot 发送过消息，否则无法推送。
- 不能向已注销或被限制发言的账号推送。
- 草稿是瞬时更新，不产生消息记录，也不支持 `reply_markup`。

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，见下方常见错误描述 |
| 401 | Token 无效或已过期 |
| 500 | 服务器内部错误 |

##### 常见错误描述

| description | 含义 |
| --- | --- |
| RANDOM\_ID\_INVALID | draft\_id 缺失或为 0 |
| MESSAGE\_EMPTY | text 为空 |
| MESSAGE\_TOO\_LONG | text 超过 4096 个字符 |
| TEXTDRAFT\_PEER\_INVALID | chat\_id 不是用户 ID（如群组、频道或字符串用户名） |
| USER\_IS\_BOT | 目标是另一个 Bot |
| USER\_DELETED | 目标账号已注销或被限制 |
| CHAT\_WRITE\_FORBIDDEN | 目标用户从未给 Bot 发过消息，无法推送草稿 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/sendMessageDraft" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "draft_id": 1720000000123,
    "text": "帮我查询今天的订单"
  }'
```

##### 带格式的草稿

```
curl -X POST "https://api.safew.bot/<token>/sendMessageDraft" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "draft_id": 1720000000124,
    "text": "查询订单 <b>NO.2024001</b>",
    "parse_mode": "HTML"
  }'
```

[返回目录](#目录) · [原网页](#page-sendmessagedraft)

---

<a id="page-sendphoto"></a>

### sendPhoto

发送图片到指定聊天。支持直接上传文件或传入已有文件的 `file_id`。

#### 请求

`POST /:token/sendPhoto`

文件上传时使用 `multipart/form-data` 编码。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| photo | String/File | 是 | 图片文件或已上传文件的 `file_id`，最大 10MB |
| caption | String | 否 | 图片说明文字 |
| parse\_mode | String | 否 | 说明文字的解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| reply\_markup | Object | 否 | 自定义键盘或内联键盘标记，JSON 序列化对象 |

#### 响应

返回发送成功的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 101,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "photo": [
      {
        "file_id": "AgACAgIAAxkBAAI...",
        "file_unique_id": "AQADAgAT...",
        "file_size": 12345,
        "width": 320,
        "height": 240
      },
      {
        "file_id": "AgACAgIAAxkBAAI...",
        "file_unique_id": "AQADAgAT...",
        "file_size": 56789,
        "width": 800,
        "height": 600
      }
    ],
    "caption": "这是一张示例图片"
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如文件格式不支持或超出大小限制（最大 10MB） |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 413 | 文件体积过大 |
| 500 | 服务器内部错误 |

#### 示例

##### 通过文件上传发送图片

```
curl -X POST "https://api.safew.bot/<token>/sendPhoto" \
  -F "chat_id=987654321" \
  -F "photo=@/path/to/image.jpg" \
  -F "caption=这是一张示例图片"
```

##### 通过 file\_id 发送图片

```
curl -X POST "https://api.safew.bot/<token>/sendPhoto" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "photo": "AgACAgIAAxkBAAI...",
    "caption": "转发一张图片"
  }'
```

[返回目录](#目录) · [原网页](#page-sendphoto)

---

<a id="page-sendvideo"></a>

### sendVideo

发送视频到指定聊天。支持直接上传文件或传入已有文件的 `file_id`。

#### 请求

`POST /:token/sendVideo`

文件上传时使用 `multipart/form-data` 编码。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| video | String/File | 是 | 视频文件或已上传文件的 `file_id` |
| caption | String | 否 | 视频说明文字 |
| parse\_mode | String | 否 | 说明文字的解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| width | Integer | 否 | 视频宽度（像素） |
| height | Integer | 否 | 视频高度（像素） |
| duration | Integer | 否 | 视频时长（秒） |
| reply\_markup | Object | 否 | 自定义键盘或内联键盘标记，JSON 序列化对象 |

#### 响应

返回发送成功的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 102,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "video": {
      "file_id": "BAACAgIAAxkBAAI...",
      "file_unique_id": "AQADAgAT...",
      "width": 1920,
      "height": 1080,
      "duration": 30,
      "file_size": 1048576
    },
    "caption": "这是一段示例视频"
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如文件格式不支持 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 413 | 文件体积过大 |
| 500 | 服务器内部错误 |

#### 示例

##### 通过文件上传发送视频

```
curl -X POST "https://api.safew.bot/<token>/sendVideo" \
  -F "chat_id=987654321" \
  -F "video=@/path/to/video.mp4" \
  -F "caption=这是一段示例视频" \
  -F "width=1920" \
  -F "height=1080" \
  -F "duration=30"
```

##### 通过 file\_id 发送视频

```
curl -X POST "https://api.safew.bot/<token>/sendVideo" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "video": "BAACAgIAAxkBAAI...",
    "caption": "转发一段视频"
  }'
```

[返回目录](#目录) · [原网页](#page-sendvideo)

---

<a id="page-sendvoice"></a>

### sendVoice

发送语音消息到指定聊天。支持直接上传文件或传入已有文件的 `file_id`。

#### 请求

`POST /:token/sendVoice`

文件上传时使用 `multipart/form-data` 编码。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| voice | String/File | 是 | 语音文件或已上传文件的 `file_id` |
| caption | String | 否 | 语音说明文字 |
| parse\_mode | String | 否 | 说明文字的解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| duration | Integer | 否 | 语音时长（秒） |
| reply\_markup | Object | 否 | 自定义键盘或内联键盘标记，JSON 序列化对象 |

#### 响应

返回发送成功的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 103,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "voice": {
      "file_id": "AwACAgIAAxkBAAI...",
      "file_unique_id": "AQADAgAT...",
      "duration": 5,
      "mime_type": "audio/ogg",
      "file_size": 23456
    },
    "caption": "这是一条语音消息"
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如文件格式不支持 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 413 | 文件体积过大 |
| 500 | 服务器内部错误 |

#### 示例

##### 通过文件上传发送语音

```
curl -X POST "https://api.safew.bot/<token>/sendVoice" \
  -F "chat_id=987654321" \
  -F "voice=@/path/to/voice.ogg" \
  -F "caption=这是一条语音消息" \
  -F "duration=5"
```

##### 通过 file\_id 发送语音

```
curl -X POST "https://api.safew.bot/<token>/sendVoice" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "voice": "AwACAgIAAxkBAAI...",
    "duration": 5
  }'
```

[返回目录](#目录) · [原网页](#page-sendvoice)

---

<a id="page-sendaudio"></a>

### sendAudio

发送音频文件到指定聊天。支持直接上传文件或传入已有文件的 `file_id`。与 `sendVoice` 不同，`sendAudio` 用于发送音乐等音频文件，客户端会以音频播放器形式展示。

#### 请求

`POST /:token/sendAudio`

文件上传时使用 `multipart/form-data` 编码。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| audio | String/File | 是 | 音频文件或已上传文件的 `file_id` |
| caption | String | 否 | 音频说明文字 |
| parse\_mode | String | 否 | 说明文字的解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| duration | Integer | 否 | 音频时长（秒） |
| reply\_markup | Object | 否 | 自定义键盘或内联键盘标记，JSON 序列化对象 |

#### 响应

返回发送成功的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 104,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "audio": {
      "file_id": "CQACAgIAAxkBAAI...",
      "file_unique_id": "AQADAgAT...",
      "duration": 180,
      "mime_type": "audio/mpeg",
      "file_size": 3145728
    },
    "caption": "这是一首歌曲"
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如文件格式不支持 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 413 | 文件体积过大 |
| 500 | 服务器内部错误 |

#### 示例

##### 通过文件上传发送音频

```
curl -X POST "https://api.safew.bot/<token>/sendAudio" \
  -F "chat_id=987654321" \
  -F "audio=@/path/to/song.mp3" \
  -F "caption=这是一首歌曲" \
  -F "duration=180"
```

##### 通过 file\_id 发送音频

```
curl -X POST "https://api.safew.bot/<token>/sendAudio" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "audio": "CQACAgIAAxkBAAI...",
    "caption": "转发一首歌曲"
  }'
```

[返回目录](#目录) · [原网页](#page-sendaudio)

---

<a id="page-senddocument"></a>

### sendDocument

发送文档文件到指定聊天。支持直接上传文件或传入已有文件的 `file_id`。适用于发送各类文件，最大支持 100MB。

#### 请求

`POST /:token/sendDocument`

文件上传时使用 `multipart/form-data` 编码。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| document | String/File | 是 | 文档文件或已上传文件的 `file_id`，最大 100MB |
| caption | String | 否 | 文档说明文字 |
| parse\_mode | String | 否 | 说明文字的解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| reply\_markup | Object | 否 | 自定义键盘或内联键盘标记，JSON 序列化对象 |

#### 响应

返回发送成功的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 105,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "document": {
      "file_id": "BQACAgIAAxkBAAI...",
      "file_unique_id": "AQADAgAT...",
      "file_name": "report.pdf",
      "mime_type": "application/pdf",
      "file_size": 524288
    },
    "caption": "这是一份报告文档"
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 document 或 chat\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 413 | 文件体积过大（超过 100MB） |
| 500 | 服务器内部错误 |

#### 示例

##### 通过文件上传发送文档

```
curl -X POST "https://api.safew.bot/<token>/sendDocument" \
  -F "chat_id=987654321" \
  -F "document=@/path/to/report.pdf" \
  -F "caption=这是一份报告文档"
```

##### 通过 file\_id 发送文档

```
curl -X POST "https://api.safew.bot/<token>/sendDocument" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "document": "BQACAgIAAxkBAAI...",
    "caption": "转发一份文档"
  }'
```

[返回目录](#目录) · [原网页](#page-senddocument)

---

<a id="page-sendmediagroup"></a>

### sendMediaGroup

发送媒体组（一组图片或视频）到指定聊天。单次最多发送 10 个媒体文件。

#### 请求

`POST /:token/sendMediaGroup`

文件上传时使用 `multipart/form-data` 编码。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| media | JSON String | 是 | InputMedia 对象数组的 JSON 字符串，最多 10 个元素 |

##### InputMedia 对象

`media` 参数是一个 JSON 数组，每个元素为 InputMedia 对象：

| 字段 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| type | String | 是 | 媒体类型，如 `photo` 或 `video` |
| media | String | 是 | 文件的 `file_id` 或通过 `attach://` 引用上传的文件 |
| caption | String | 否 | 媒体说明文字 |
| parse\_mode | String | 否 | 说明文字的解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |

#### 响应

返回发送成功的 Message 对象数组。

```
{
  "ok": true,
  "result": [
    {
      "message_id": 106,
      "from": {
        "id": 123456789,
        "is_bot": true,
        "first_name": "MyBot",
        "username": "my_bot"
      },
      "chat": {
        "id": 987654321,
        "first_name": "User",
        "username": "user123",
        "type": "private"
      },
      "date": 1700000000,
      "media_group_id": "13579246810",
      "photo": [
        {
          "file_id": "AgACAgIAAxkBAAI...",
          "file_unique_id": "AQADAgAT...",
          "file_size": 12345,
          "width": 800,
          "height": 600
        }
      ],
      "caption": "第一张图片"
    },
    {
      "message_id": 107,
      "from": {
        "id": 123456789,
        "is_bot": true,
        "first_name": "MyBot",
        "username": "my_bot"
      },
      "chat": {
        "id": 987654321,
        "first_name": "User",
        "username": "user123",
        "type": "private"
      },
      "date": 1700000000,
      "media_group_id": "13579246810",
      "photo": [
        {
          "file_id": "AgACAgIAAxkBAAI...",
          "file_unique_id": "AQADAgAT...",
          "file_size": 67890,
          "width": 800,
          "height": 600
        }
      ]
    }
  ]
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 media 格式不正确或超过 10 个元素 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 413 | 文件体积过大 |
| 500 | 服务器内部错误 |

#### 示例

##### 通过 file\_id 发送媒体组

```
curl -X POST "https://api.safew.bot/<token>/sendMediaGroup" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "media": [
      {
        "type": "photo",
        "media": "AgACAgIAAxkBAAI_photo1...",
        "caption": "第一张图片"
      },
      {
        "type": "photo",
        "media": "AgACAgIAAxkBAAI_photo2..."
      },
      {
        "type": "photo",
        "media": "AgACAgIAAxkBAAI_photo3..."
      }
    ]
  }'
```

##### 通过文件上传发送媒体组

```
curl -X POST "https://api.safew.bot/<token>/sendMediaGroup" \
  -F "chat_id=987654321" \
  -F 'media=[{"type":"photo","media":"attach://photo1","caption":"第一张"},{"type":"photo","media":"attach://photo2"}]' \
  -F "photo1=@/path/to/image1.jpg" \
  -F "photo2=@/path/to/image2.jpg"
```

[返回目录](#目录) · [原网页](#page-sendmediagroup)

---

<a id="page-senddice"></a>

### sendDice

发送一个带随机结果的动画表情消息，例如骰子、飞镖、篮球、足球、老虎机或保龄球。

#### 请求

`POST /:token/sendDice`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符，数字 ID 或数字字符串。当前不支持 `@username` 形式 |
| emoji | String | 否 | 动画表情，可选 `🎲`、`🎯`、`🏀`、`⚽`、`🎰`、`🎳`，默认 `🎲` |
| message\_thread\_id | Integer | 否 | 目标消息线程（话题）的唯一标识符，仅用于话题群 |
| reply\_parameters | Object | 否 | 回复参数。当前仅 `message_id` 字段生效，该消息将作为对指定消息的回复发送 |
| reply\_markup | Object | 否 | 自定义键盘或内联键盘标记，支持 JSON 序列化对象或字符串两种格式 |

以下参数可以传入，但当前实现尚未生效，会被服务端忽略：`business_connection_id`、`disable_notification`、`protect_content`、`allow_paid_broadcast`、`message_effect_id`。

#### 随机值范围

点数由服务端随机生成，调用方无法指定。

| emoji | 类型 | value 范围 |
| --- | --- | --- |
| `🎲` | 骰子 | 1-6 |
| `🎯` | 飞镖 | 1-6 |
| `🎳` | 保龄球 | 1-6 |
| `🏀` | 篮球 | 1-5 |
| `⚽` | 足球 | 1-5 |
| `🎰` | 老虎机 | 1-64 |

足球同时接受带变体选择符的 `⚽️`，效果与 `⚽` 一致。传入表格之外的表情返回 `EMOTICON_STICKERPACK_MISSING`。

#### 响应

返回发送成功的 Message 对象，投掷结果在 `dice` 字段中：`emoji` 回显实际使用的表情，`value` 为服务端随机生成的点数。

```
{
  "ok": true,
  "result": {
    "message_id": 102,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "dice": {
      "emoji": "🎲",
      "value": 4
    }
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id、emoji 不支持或 reply\_markup 格式错误；触发发送频率限制时同样返回 400，描述为 `MESSAGE_TOO_MUCH` |
| 401 | Token 无效或已过期 |
| 403 | Bot 被该用户封禁或无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### 发送默认骰子

```
curl -X POST "https://api.safew.bot/<token>/sendDice" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321
  }'
```

##### 发送篮球动画

```
curl -X POST "https://api.safew.bot/<token>/sendDice" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "emoji": "🏀"
  }'
```

##### 带内联键盘的小游戏

```
curl -X POST "https://api.safew.bot/<token>/sendDice" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "emoji": "🎯",
    "reply_markup": {
      "inline_keyboard": [
        [
          {"text": "再来一次", "callback_data": "dice_retry"},
          {"text": "查看规则", "callback_data": "dice_rules"}
        ]
      ]
    }
  }'
```

#### 相关应用

- **随机小游戏**：在群聊或私聊中发起骰子、飞镖、篮球、足球、老虎机、保龄球等轻量互动。
- **抽奖与决策**：用 `dice.value` 作为随机结果，配合业务规则实现抽奖、排名或随机选择。
- **竞猜挑战**：用户先提交预测，再由 Bot 调用 `sendDice` 生成公开随机结果。
- **群内活跃**：配合 `reply_markup` 提供“再来一次”“查看规则”等按钮，形成连续互动。

[返回目录](#目录) · [原网页](#page-senddice)

---

<a id="page-editmessagetext"></a>

### editMessageText

编辑已发送消息的文本内容。

#### 请求

`POST /:token/editMessageText`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 条件必填 | 目标聊天的唯一标识符或用户名。若未指定 `inline_message_id` 则必填 |
| message\_id | Integer | 条件必填 | 要编辑的消息 ID。若未指定 `inline_message_id` 则必填 |
| inline\_message\_id | String | 条件必填 | 内联消息的标识符。若未指定 `chat_id` 和 `message_id` 则必填 |
| text | String | 是 | 新的消息文本内容 |
| parse\_mode | String | 否 | 消息文本解析模式，支持 `MarkdownV2`、`Markdown`、`HTML`，详见[消息格式化](#page-formatting) |
| entities | MessageEntity[] | 否 | 消息文本中的特殊实体列表，可替代 `parse_mode` 使用 |
| link\_preview\_options | Object | 否 | 链接预览生成选项 |
| reply\_markup | Object | 否 | 新的内联键盘标记，JSON 序列化对象 |

#### 响应

返回编辑后的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 100,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "edit_date": 1700000060,
    "text": "已更新的消息内容"
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如消息内容未发生变化或缺少必填参数 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权编辑该消息（只能编辑 Bot 自己发送的消息） |
| 404 | 消息不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/editMessageText" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "message_id": 100,
    "text": "已更新的消息内容",
    "parse_mode": "HTML"
  }'
```

##### 编辑消息并更新内联键盘

```
curl -X POST "https://api.safew.bot/<token>/editMessageText" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "message_id": 100,
    "text": "请重新选择：",
    "reply_markup": {
      "inline_keyboard": [
        [
          {"text": "新选项 A", "callback_data": "new_option_a"},
          {"text": "新选项 B", "callback_data": "new_option_b"}
        ]
      ]
    }
  }'
```

[返回目录](#目录) · [原网页](#page-editmessagetext)

---

<a id="page-editmessagereplymarkup"></a>

### editMessageReplyMarkup

编辑已发送消息的回复标记（内联键盘）。仅修改键盘部分，不影响消息文本内容。

#### 请求

`POST /:token/editMessageReplyMarkup`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 条件必填 | 目标聊天的唯一标识符或用户名。若未指定 `inline_message_id` 则必填 |
| message\_id | Integer | 条件必填 | 要编辑的消息 ID。若未指定 `inline_message_id` 则必填 |
| inline\_message\_id | String | 条件必填 | 内联消息的标识符。若未指定 `chat_id` 和 `message_id` 则必填 |
| reply\_markup | Object | 否 | 新的内联键盘标记，JSON 序列化对象。不传或传空对象则移除现有键盘 |

#### 响应

返回编辑后的 Message 对象。

```
{
  "ok": true,
  "result": {
    "message_id": 100,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "first_name": "User",
      "username": "user123",
      "type": "private"
    },
    "date": 1700000000,
    "edit_date": 1700000120,
    "text": "请选择一个选项：",
    "reply_markup": {
      "inline_keyboard": [
        [
          {"text": "新选项 A", "callback_data": "new_a"},
          {"text": "新选项 B", "callback_data": "new_b"}
        ]
      ]
    }
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 reply\_markup 格式不正确或键盘未发生变化 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权编辑该消息（只能编辑 Bot 自己发送的消息） |
| 404 | 消息不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### 更新内联键盘

```
curl -X POST "https://api.safew.bot/<token>/editMessageReplyMarkup" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "message_id": 100,
    "reply_markup": {
      "inline_keyboard": [
        [
          {"text": "新选项 A", "callback_data": "new_a"},
          {"text": "新选项 B", "callback_data": "new_b"}
        ],
        [
          {"text": "取消", "callback_data": "cancel"}
        ]
      ]
    }
  }'
```

##### 移除内联键盘

```
curl -X POST "https://api.safew.bot/<token>/editMessageReplyMarkup" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "message_id": 100,
    "reply_markup": {}
  }'
```

[返回目录](#目录) · [原网页](#page-editmessagereplymarkup)

---

<a id="page-deletemessage"></a>

### deleteMessage

删除一条消息。Bot 只能删除 48 小时内的消息。

#### 请求

`POST /:token/deletemessage`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天 ID |
| message\_id | Integer | 是 | 要删除的消息 ID |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效 |
| 403 | 无权限执行此操作 |
| 404 | 消息不存在或已超过 48 小时 |

[返回目录](#目录) · [原网页](#page-deletemessage)

---

<a id="page-deletemessages"></a>

### deleteMessages

批量删除消息，最多 100 条。

逐条删除：其中某几条 `message_id` 无效（不存在、已删除、不属于该会话），或 bot 无权删除（消息不是 bot 发的，且 bot 不是管理员），都**不会**阻止其余消息被删除。

#### 请求

`POST /:token/deletemessages`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天 ID |
| message\_ids | Integer[] | 是 | 要删除的消息 ID 列表（最多 100 条） |
| return\_failed | Boolean | 否 | 为 `true` 时返回逐条删除结果，默认 `false`（只返回 `true`） |

#### 响应

不传 `return_failed` 时，成功返回 Boolean 值 `true`，与旧版本一致：

```
{
  "ok": true,
  "result": true
}
```

传 `return_failed: true` 时，`result` 返回逐条结果：

```
{
  "ok": true,
  "result": {
    "deleted_message_ids": [101, 103],
    "failed_message_ids": [
      { "message_id": 102, "description": "message to delete not found" },
      { "message_id": 104, "description": "not enough rights to delete message" }
    ]
  }
}
```

##### FailedMessageId

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| message\_id | Integer | 删除失败的消息 ID |
| description | String | 失败原因 |

`description` 的取值：

| 取值 | 含义 |
| --- | --- |
| message to delete not found | 消息不存在、已被删除，或不属于 `chat_id` 指定的会话 |
| not enough rights to delete message | 消息不是 bot 发的，且 bot 在该群组/频道不是管理员 |
| chat not found | `chat_id` 既不是用户也不是群组/频道 |

注意

一条消息都没删成时接口仍返回成功（`ok: true`），失败原因通过 `failed_message_ids` 给出。 如果需要「消息不存在就报错」的严格语义，请改用单条删除的 [deleteMessage](#page-deletemessage)。

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误（如 `message_ids` 超过 100 条） |
| 401 | Token 无效 |
| 403 | 无权限执行此操作（bot 被禁言，或群组受限） |

#### cURL 示例

```
curl -X POST "https://api.safew.bot/bot<token>/deleteMessages" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 123456789,
    "message_ids": [101, 102, 103],
    "return_failed": true
  }'
```

[返回目录](#目录) · [原网页](#page-deletemessages)

---

<a id="page-copymessage"></a>

### copyMessage

复制消息到另一个聊天。

#### 请求

`POST /:token/copymessage`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天 ID |
| from\_chat\_id | Integer/String | 是 | 源聊天 ID |
| message\_id | Integer | 是 | 要复制的消息 ID |
| message\_thread\_id | Integer | 否 | 目标消息线程（话题）的唯一标识符，仅用于超级群和频道 |
| caption | String | 否 | 新的说明文本，替换原消息的文本/说明。按纯文本处理，原消息的格式实体会被清除 |
| parse\_mode | String | 否 | 当前版本不生效：`caption` 不做格式解析（各格式风格见[消息格式化](#page-formatting)） |
| caption\_entities | MessageEntity[] | 否 | 当前版本不生效 |
| show\_caption\_above\_media | Boolean | 否 | 是否在媒体上方显示说明文本 |
| video\_start\_timestamp | Integer | 否 | 视频开始播放的时间戳（秒） |
| disable\_notification | Boolean | 否 | 静默发送消息，用户将收到无声通知 |
| protect\_content | Boolean | 否 | 保护消息内容不被转发和保存 |
| reply\_parameters | Object | 否 | 回复参数，描述要回复的消息 |
| reply\_to\_message\_id | Integer | 否 | 要回复的消息 ID |
| reply\_markup | Object | 否 | 自定义回复标记 |

#### 响应

成功时返回 MessageId 对象数组。

```
{
  "ok": true,
  "result": [
    {
      "message_id": 12345
    }
  ]
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效 |
| 403 | 无权限执行此操作 |
| 404 | 源消息不存在 |

[返回目录](#目录) · [原网页](#page-copymessage)

---

<a id="page-pinchatmessage"></a>

### pinChatMessage

将消息钉在聊天中。

#### 请求

`POST /:token/pinchatmessage`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天 ID |
| message\_id | Integer | 是 | 要钉住的消息 ID |
| disable\_notification | Boolean | 否 | 是否静默钉住 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效 |
| 403 | 无权限执行此操作 |
| 404 | 消息不存在 |

[返回目录](#目录) · [原网页](#page-pinchatmessage)

---

<a id="page-unpinchatmessage"></a>

### unpinChatMessage

取消钉住聊天中的消息。

#### 请求

`POST /:token/unpinchatmessage`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天 ID |
| message\_id | Integer | 否 | 要取消钉住的消息 ID |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效 |
| 403 | 无权限执行此操作 |
| 404 | 消息不存在或未被钉住 |

[返回目录](#目录) · [原网页](#page-unpinchatmessage)

---

## Webhook

<a id="page-setwebhook"></a>

### setWebhook

设置 Webhook URL 以接收更新。设置后，getUpdates 将不再工作。支持上传自签名证书。

#### 请求

`POST /:token/setwebhook`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| url | String | 是 | Webhook URL（HTTPS） |
| certificate | File | 否 | 自签名 SSL 证书（PEM 格式） |
| ip\_address | String | 否 | 固定 IP 地址 |
| max\_connections | Integer | 否 | 最大并发连接数 |
| allowed\_updates | String[] | 否 | 允许的更新类型 |
| secret\_token | String | 否 | 用于验证请求来源的密钥 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误或 URL 格式无效 |
| 401 | Token 无效 |
| 403 | 无权限执行此操作 |

[返回目录](#目录) · [原网页](#page-setwebhook)

---

<a id="page-deletewebhook"></a>

### deleteWebhook

删除已设置的 Webhook。

#### 请求

`POST /:token/deletewebhook`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| drop\_pending\_updates | Boolean | 否 | 是否丢弃待处理的更新 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效 |
| 403 | 无权限执行此操作 |

[返回目录](#目录) · [原网页](#page-deletewebhook)

---

<a id="page-getwebhookinfo"></a>

### getWebhookInfo

获取当前 Webhook 的配置信息。

#### 请求

`POST /:token/getwebhookinfo`

#### 参数

无参数。

#### 响应

成功时返回 WebhookInfo 对象，包含 `url`、`has_custom_certificate`、`pending_update_count` 等字段。

```
{
  "ok": true,
  "result": {
    "url": "https://example.com/webhook",
    "has_custom_certificate": false,
    "pending_update_count": 0,
    "ip_address": "1.2.3.4",
    "max_connections": 40,
    "allowed_updates": ["message", "callback_query"],
    "last_error_date": null,
    "last_error_message": null
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效 |
| 403 | 无权限执行此操作 |

[返回目录](#目录) · [原网页](#page-getwebhookinfo)

---

## 聊天管理

<a id="page-getchat"></a>

### getChat

获取聊天的详细信息。

#### 请求

`POST /:token/getchat`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |

#### 响应

返回 Chat 对象。

```
{
  "ok": true,
  "result": {
    "id": -1001234567890,
    "type": "supergroup",
    "title": "技术交流群",
    "username": "tech_group",
    "first_name": null,
    "description": "这是一个技术交流群组",
    "permissions": {
      "can_send_messages": true,
      "can_send_media_messages": true,
      "can_send_polls": true,
      "can_send_other_messages": true,
      "can_add_web_page_previews": true,
      "can_change_info": false,
      "can_invite_users": true,
      "can_pin_messages": false
    }
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| id | Integer | 聊天的唯一标识符 |
| type | String | 聊天类型，可选值：`private`、`group`、`supergroup`、`channel` |
| title | String | 群组或频道的标题，私聊时为空 |
| username | String | 聊天的用户名（如果设置了） |
| first\_name | String | 私聊时对方的名字 |
| description | String | 群组或频道的简介描述 |
| permissions | ChatPermissions | 群组的默认成员权限 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 chat\_id 格式不正确 |
| 401 | Token 无效或已过期 |
| 403 | Bot 不是该聊天的成员，无权获取信息 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/getchat" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890
  }'
```

[返回目录](#目录) · [原网页](#page-getchat)

---

<a id="page-getchatmembercount"></a>

### getChatMemberCount

获取聊天中的成员数量。

#### 请求

`POST /:token/getchatmembercount`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |

#### 响应

返回聊天成员数量（Integer）。

```
{
  "ok": true,
  "result": 256
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 chat\_id 格式不正确 |
| 401 | Token 无效或已过期 |
| 403 | Bot 不是该聊天的成员，无权获取信息 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/getchatmembercount" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890
  }'
```

[返回目录](#目录) · [原网页](#page-getchatmembercount)

---

<a id="page-getchatmember"></a>

### getChatMember

获取聊天中某个成员的信息。

#### 请求

`POST /:token/getchatmember`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 目标用户的唯一标识符 |

#### 响应

返回 ChatMember 对象。

```
{
  "ok": true,
  "result": {
    "user": {
      "id": 123456789,
      "is_bot": false,
      "first_name": "张三",
      "username": "zhangsan"
    },
    "status": "administrator",
    "custom_title": "技术负责人",
    "is_anonymous": false,
    "can_manage_chat": true,
    "can_post_messages": true,
    "can_edit_messages": true,
    "can_delete_messages": true,
    "can_manage_video_chats": true,
    "can_restrict_members": true,
    "can_promote_members": false,
    "can_change_info": true,
    "can_invite_users": true,
    "can_pin_messages": true
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| user | User | 成员的用户信息对象 |
| status | String | 成员状态，可选值：`creator`、`administrator`、`member`、`restricted`、`left`、`kicked` |
| custom\_title | String | 管理员的自定义头衔 |
| is\_anonymous | Boolean | 管理员是否匿名 |
| can\_manage\_chat | Boolean | 是否可以管理聊天 |
| can\_post\_messages | Boolean | 是否可以发布消息（仅频道） |
| can\_edit\_messages | Boolean | 是否可以编辑消息（仅频道） |
| can\_delete\_messages | Boolean | 是否可以删除消息 |
| can\_manage\_video\_chats | Boolean | 是否可以管理视频聊天 |
| can\_restrict\_members | Boolean | 是否可以限制成员 |
| can\_promote\_members | Boolean | 是否可以提升成员为管理员 |
| can\_change\_info | Boolean | 是否可以修改聊天信息 |
| can\_invite\_users | Boolean | 是否可以邀请用户 |
| can\_pin\_messages | Boolean | 是否可以置顶消息 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id 或 user\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 不是该聊天的成员，无权获取信息 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/getchatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 123456789
  }'
```

[返回目录](#目录) · [原网页](#page-getchatmember)

---

<a id="page-getchatadministrators"></a>

### getChatAdministrators

获取聊天中所有管理员列表。

#### 请求

`POST /:token/getchatadministrators`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |

#### 响应

返回 ChatMember 对象数组。

```
{
  "ok": true,
  "result": [
    {
      "user": {
        "id": 100000001,
        "is_bot": false,
        "first_name": "群主",
        "username": "owner"
      },
      "status": "creator",
      "custom_title": "创建者",
      "is_anonymous": false
    },
    {
      "user": {
        "id": 100000002,
        "is_bot": false,
        "first_name": "管理员A",
        "username": "admin_a"
      },
      "status": "administrator",
      "custom_title": "技术负责人",
      "is_anonymous": false,
      "can_manage_chat": true,
      "can_post_messages": true,
      "can_edit_messages": true,
      "can_delete_messages": true,
      "can_manage_video_chats": true,
      "can_restrict_members": true,
      "can_promote_members": false,
      "can_change_info": true,
      "can_invite_users": true,
      "can_pin_messages": true
    },
    {
      "user": {
        "id": 123456789,
        "is_bot": true,
        "first_name": "MyBot",
        "username": "my_bot"
      },
      "status": "administrator",
      "custom_title": "",
      "is_anonymous": false,
      "can_manage_chat": true,
      "can_post_messages": false,
      "can_edit_messages": false,
      "can_delete_messages": true,
      "can_manage_video_chats": false,
      "can_restrict_members": true,
      "can_promote_members": false,
      "can_change_info": false,
      "can_invite_users": true,
      "can_pin_messages": true
    }
  ]
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 chat\_id 格式不正确 |
| 401 | Token 无效或已过期 |
| 403 | Bot 不是该聊天的成员，无权获取信息 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/getchatadministrators" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890
  }'
```

[返回目录](#目录) · [原网页](#page-getchatadministrators)

---

<a id="page-setchattitle"></a>

### setChatTitle

设置聊天标题。Bot 需要管理员权限。

#### 请求

`POST /:token/setchattitle`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| title | String | 是 | 新的聊天标题，长度 1-255 字符 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 title 为空或超过 255 字符 |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有管理员权限或没有修改聊天信息的权限 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/setchattitle" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "title": "新的群组标题"
  }'
```

[返回目录](#目录) · [原网页](#page-setchattitle)

---

<a id="page-leavechat"></a>

### leaveChat

Bot 离开聊天群组。

#### 请求

`POST /:token/leavechat`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 chat\_id 格式不正确 |
| 401 | Token 无效或已过期 |
| 403 | Bot 不是该聊天的成员 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/leavechat" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890
  }'
```

[返回目录](#目录) · [原网页](#page-leavechat)

---

<a id="page-setchatpermissions"></a>

### setChatPermissions

设置群组的默认权限。Bot 需要管理员权限。

#### 请求

`POST /:token/setchatpermissions`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| permissions | ChatPermissions | 是 | 群组的默认权限对象 |

##### ChatPermissions 对象

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| can\_send\_messages | Boolean | 是否允许发送文本消息、联系人、位置和场所 |
| can\_send\_media\_messages | Boolean | 是否允许发送音频、文档、照片、视频、视频备注和语音备注 |
| can\_send\_polls | Boolean | 是否允许发送投票 |
| can\_send\_other\_messages | Boolean | 是否允许发送动画、游戏、贴纸以及使用内联 Bot |
| can\_add\_web\_page\_previews | Boolean | 是否允许添加网页链接预览 |
| can\_change\_info | Boolean | 是否允许更改聊天标题、照片和其他设置 |
| can\_invite\_users | Boolean | 是否允许邀请新用户加入聊天 |
| can\_pin\_messages | Boolean | 是否允许置顶消息 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 permissions 格式不正确 |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有管理员权限或没有修改权限的权限 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/setchatpermissions" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "permissions": {
      "can_send_messages": true,
      "can_send_media_messages": true,
      "can_send_polls": false,
      "can_send_other_messages": true,
      "can_add_web_page_previews": true,
      "can_change_info": false,
      "can_invite_users": true,
      "can_pin_messages": false
    }
  }'
```

[返回目录](#目录) · [原网页](#page-setchatpermissions)

---

## 成员管理

<a id="page-banchatmember"></a>

### banChatMember

将用户从聊天中踢出。

#### 请求

`POST /:token/banchatmember`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 要封禁的用户的唯一标识符 |
| until\_date | Integer | 否 | 封禁截止时间的 Unix 时间戳。若为 0 或不设置，则永久封禁。封禁时间少于 30 秒或多于 366 天将被视为永久封禁 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id 或 user\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有管理员权限或没有封禁用户的权限 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

###### 永久封禁用户

```
curl -X POST "https://api.safew.bot/<token>/banchatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321
  }'
```

###### 临时封禁用户（24 小时）

```
curl -X POST "https://api.safew.bot/<token>/banchatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "until_date": 1700086400
  }'
```

[返回目录](#目录) · [原网页](#page-banchatmember)

---

<a id="page-unbanchatmember"></a>

### unbanChatMember

解除用户的封禁。

#### 请求

`POST /:token/unbanchatmember`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 要解封的用户的唯一标识符 |
| only\_if\_banned | Boolean | 否 | 如果设为 `true`，则仅在用户已被封禁时才执行解封操作。避免将未封禁用户意外移出群聊 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id 或 user\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有管理员权限或没有解封用户的权限 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

###### 解封用户

```
curl -X POST "https://api.safew.bot/<token>/unbanchatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321
  }'
```

###### 仅在已封禁时解封

```
curl -X POST "https://api.safew.bot/<token>/unbanchatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "only_if_banned": true
  }'
```

[返回目录](#目录) · [原网页](#page-unbanchatmember)

---

<a id="page-banchatsenderchat"></a>

### banChatSenderChat

禁止某个频道在聊天中发送消息。

#### 请求

`POST /:token/banchatsenderchat`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| sender\_chat\_id | Integer | 是 | 要禁止的频道的唯一标识符 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id 或 sender\_chat\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有管理员权限或没有封禁频道的权限 |
| 404 | 聊天或频道不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/banchatsenderchat" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "sender_chat_id": -1009876543210
  }'
```

[返回目录](#目录) · [原网页](#page-banchatsenderchat)

---

<a id="page-unbanchatsenderchat"></a>

### unbanChatSenderChat

解除频道的发送禁止。

#### 请求

`POST /:token/unbanchatsenderchat`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| sender\_chat\_id | Integer | 是 | 要解除禁止的频道的唯一标识符 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id 或 sender\_chat\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有管理员权限或没有解除频道禁止的权限 |
| 404 | 聊天或频道不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/unbanchatsenderchat" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "sender_chat_id": -1009876543210
  }'
```

[返回目录](#目录) · [原网页](#page-unbanchatsenderchat)

---

<a id="page-promotechatmember"></a>

### promoteChatMember

提升或降级聊天成员为管理员。

#### 请求

`POST /:token/promotechatmember`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 目标用户的唯一标识符 |
| is\_anonymous | Boolean | 否 | 是否允许管理员匿名操作 |
| can\_manage\_chat | Boolean | 否 | 是否可以管理聊天（访问聊天统计数据、查看频道成员列表等） |
| can\_post\_messages | Boolean | 否 | 是否可以在频道中发布消息（仅频道） |
| can\_edit\_messages | Boolean | 否 | 是否可以编辑频道中的消息（仅频道） |
| can\_delete\_messages | Boolean | 否 | 是否可以删除消息 |
| can\_manage\_video\_chats | Boolean | 否 | 是否可以管理视频聊天 |
| can\_restrict\_members | Boolean | 否 | 是否可以限制、封禁或解封成员 |
| can\_promote\_members | Boolean | 否 | 是否可以添加新的管理员 |
| can\_change\_info | Boolean | 否 | 是否可以修改聊天标题、照片和其他设置 |
| can\_invite\_users | Boolean | 否 | 是否可以邀请新用户加入聊天 |
| can\_pin\_messages | Boolean | 否 | 是否可以置顶消息 |

> 所有可选的权限参数默认值为 `false`。传入 `true` 表示授予该权限，传入 `false` 表示撤销该权限。若需将管理员降级为普通成员，可将所有权限参数设为 `false`。

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id 或 user\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有足够的管理员权限来执行此操作 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

###### 提升为管理员（授予部分权限）

```
curl -X POST "https://api.safew.bot/<token>/promotechatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "can_manage_chat": true,
    "can_delete_messages": true,
    "can_restrict_members": true,
    "can_invite_users": true,
    "can_pin_messages": true
  }'
```

###### 降级为普通成员

```
curl -X POST "https://api.safew.bot/<token>/promotechatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "can_manage_chat": false,
    "can_delete_messages": false,
    "can_restrict_members": false,
    "can_invite_users": false,
    "can_pin_messages": false
  }'
```

[返回目录](#目录) · [原网页](#page-promotechatmember)

---

<a id="page-restrictchatmember"></a>

### restrictChatMember

限制聊天成员的权限。

#### 请求

`POST /:token/restrictchatmember`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 目标用户的唯一标识符 |
| permissions | ChatPermissions | 是 | 用户的新权限对象 |
| until\_date | Integer | 否 | 限制截止时间的 Unix 时间戳。若为 0 或不设置，则永久限制。限制时间少于 30 秒或多于 366 天将被视为永久限制 |

##### ChatPermissions 对象

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| can\_send\_messages | Boolean | 是否允许发送文本消息、联系人、位置和场所 |
| can\_send\_media\_messages | Boolean | 是否允许发送音频、文档、照片、视频、视频备注和语音备注 |
| can\_send\_polls | Boolean | 是否允许发送投票 |
| can\_send\_other\_messages | Boolean | 是否允许发送动画、游戏、贴纸以及使用内联 Bot |
| can\_add\_web\_page\_previews | Boolean | 是否允许添加网页链接预览 |
| can\_change\_info | Boolean | 是否允许更改聊天标题、照片和其他设置 |
| can\_invite\_users | Boolean | 是否允许邀请新用户加入聊天 |
| can\_pin\_messages | Boolean | 是否允许置顶消息 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少必填参数或 permissions 格式不正确 |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有管理员权限或没有限制成员的权限 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

###### 禁言用户（禁止发送所有消息）

```
curl -X POST "https://api.safew.bot/<token>/restrictchatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "permissions": {
      "can_send_messages": false,
      "can_send_media_messages": false,
      "can_send_polls": false,
      "can_send_other_messages": false,
      "can_add_web_page_previews": false,
      "can_change_info": false,
      "can_invite_users": false,
      "can_pin_messages": false
    }
  }'
```

###### 临时限制用户（仅允许发送文本消息，持续 24 小时）

```
curl -X POST "https://api.safew.bot/<token>/restrictchatmember" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "permissions": {
      "can_send_messages": true,
      "can_send_media_messages": false,
      "can_send_polls": false,
      "can_send_other_messages": false,
      "can_add_web_page_previews": false,
      "can_change_info": false,
      "can_invite_users": false,
      "can_pin_messages": false
    },
    "until_date": 1700086400
  }'
```

[返回目录](#目录) · [原网页](#page-restrictchatmember)

---

<a id="page-setmydefaultadministratorrights"></a>

### setMyDefaultAdministratorRights

设置 Bot 被添加为管理员时的默认权限。

#### 请求

`POST /:token/setmydefaultadministratorrights`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| rights | ChatAdministratorRights | 否 | Bot 的默认管理员权限对象。如果不指定，则清除已设置的默认权限 |
| for\_channels | Boolean | 否 | 如果设为 `true`，则设置频道中的默认管理员权限；否则设置群组和超级群组中的默认管理员权限。默认值为 `false` |

##### ChatAdministratorRights 对象

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| is\_anonymous | Boolean | 是否允许管理员匿名操作 |
| can\_manage\_chat | Boolean | 是否可以管理聊天 |
| can\_delete\_messages | Boolean | 是否可以删除消息 |
| can\_manage\_video\_chats | Boolean | 是否可以管理视频聊天 |
| can\_restrict\_members | Boolean | 是否可以限制成员 |
| can\_promote\_members | Boolean | 是否可以提升成员为管理员 |
| can\_change\_info | Boolean | 是否可以修改聊天信息 |
| can\_invite\_users | Boolean | 是否可以邀请用户 |
| can\_post\_messages | Boolean | 是否可以在频道中发布消息（仅频道） |
| can\_edit\_messages | Boolean | 是否可以编辑频道中的消息（仅频道） |
| can\_pin\_messages | Boolean | 是否可以置顶消息（仅群组和超级群组） |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 rights 格式不正确 |
| 401 | Token 无效或已过期 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

###### 设置群组默认管理员权限

```
curl -X POST "https://api.safew.bot/<token>/setmydefaultadministratorrights" \
  -H "Content-Type: application/json" \
  -d '{
    "rights": {
      "is_anonymous": false,
      "can_manage_chat": true,
      "can_delete_messages": true,
      "can_manage_video_chats": false,
      "can_restrict_members": true,
      "can_promote_members": false,
      "can_change_info": false,
      "can_invite_users": true,
      "can_pin_messages": true
    }
  }'
```

###### 设置频道默认管理员权限

```
curl -X POST "https://api.safew.bot/<token>/setmydefaultadministratorrights" \
  -H "Content-Type: application/json" \
  -d '{
    "rights": {
      "is_anonymous": false,
      "can_manage_chat": true,
      "can_delete_messages": true,
      "can_post_messages": true,
      "can_edit_messages": true,
      "can_invite_users": true
    },
    "for_channels": true
  }'
```

###### 清除默认管理员权限

```
curl -X POST "https://api.safew.bot/<token>/setmydefaultadministratorrights" \
  -H "Content-Type: application/json" \
  -d '{}'
```

[返回目录](#目录) · [原网页](#page-setmydefaultadministratorrights)

---

<a id="page-setchatadministratorcustomtitle"></a>

### setChatAdministratorCustomTitle

设置管理员的自定义头衔。

#### 请求

`POST /:token/setchatadministratorcustomtitle`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 目标管理员用户的唯一标识符 |
| custom\_title | String | 是 | 管理员的自定义头衔，长度 0-16 字符。设为空字符串可清除自定义头衔 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 custom\_title 超过 16 字符或目标用户不是管理员 |
| 401 | Token 无效或已过期 |
| 403 | Bot 没有足够的管理员权限来执行此操作 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

###### 设置自定义头衔

```
curl -X POST "https://api.safew.bot/<token>/setchatadministratorcustomtitle" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "custom_title": "技术负责人"
  }'
```

###### 清除自定义头衔

```
curl -X POST "https://api.safew.bot/<token>/setchatadministratorcustomtitle" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1001234567890,
    "user_id": 987654321,
    "custom_title": ""
  }'
```

[返回目录](#目录) · [原网页](#page-setchatadministratorcustomtitle)

---

## 邀请链接

<a id="page-createchatinvitelink"></a>

### createChatInviteLink

创建群组邀请链接。

#### 请求

`POST /:token/createchatinvitelink`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| name | String | 否 | 链接名称，最多 32 个字符 |
| expire\_date | Integer | 否 | 链接过期时间，Unix 时间戳 |
| member\_limit | Integer | 否 | 通过该链接可加入的最大成员数，1-99999 |
| creates\_join\_request | Boolean | 否 | 是否需要管理员审批才能加入 |

#### 响应

返回创建成功的 ChatInviteLink 对象。

```
{
  "ok": true,
  "result": {
    "invite_link": "https://t.me/+abc123def456",
    "creator": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "creates_join_request": false,
    "is_primary": false,
    "is_revoked": false,
    "name": "推广链接",
    "expire_date": 1700000000,
    "member_limit": 100,
    "pending_join_request_count": 0
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| invite\_link | String | 邀请链接 URL |
| creator | User | 创建该链接的用户对象 |
| creates\_join\_request | Boolean | 是否需要管理员审批 |
| is\_primary | Boolean | 是否为主邀请链接 |
| is\_revoked | Boolean | 是否已被撤销 |
| name | String | 链接名称 |
| expire\_date | Integer | 过期时间，Unix 时间戳 |
| member\_limit | Integer | 最大成员数限制 |
| pending\_join\_request\_count | Integer | 待审批的加群请求数量 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 member\_limit 超出范围 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权在该聊天中创建邀请链接 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-createchatinvitelink)

---

<a id="page-editchatinvitelink"></a>

### editChatInviteLink

编辑群组邀请链接。

#### 请求

`POST /:token/editchatinvitelink`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| invite\_link | String | 是 | 要编辑的邀请链接 |
| name | String | 否 | 链接名称，最多 32 个字符 |
| expire\_date | Integer | 否 | 链接过期时间，Unix 时间戳 |
| member\_limit | Integer | 否 | 通过该链接可加入的最大成员数，1-99999 |
| creates\_join\_request | Boolean | 否 | 是否需要管理员审批才能加入 |

#### 响应

返回编辑后的 ChatInviteLink 对象。

```
{
  "ok": true,
  "result": {
    "invite_link": "https://t.me/+abc123def456",
    "creator": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "creates_join_request": false,
    "is_primary": false,
    "is_revoked": false,
    "name": "更新后的链接名称",
    "expire_date": 1700100000,
    "member_limit": 200,
    "pending_join_request_count": 0
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| invite\_link | String | 邀请链接 URL |
| creator | User | 创建该链接的用户对象 |
| creates\_join\_request | Boolean | 是否需要管理员审批 |
| is\_primary | Boolean | 是否为主邀请链接 |
| is\_revoked | Boolean | 是否已被撤销 |
| name | String | 链接名称 |
| expire\_date | Integer | 过期时间，Unix 时间戳 |
| member\_limit | Integer | 最大成员数限制 |
| pending\_join\_request\_count | Integer | 待审批的加群请求数量 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 invite\_link 格式无效或 member\_limit 超出范围 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权编辑该邀请链接 |
| 404 | 聊天或邀请链接不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-editchatinvitelink)

---

<a id="page-revokechatinvitelink"></a>

### revokeChatInviteLink

撤销群组邀请链接。

#### 请求

`POST /:token/revokechatinvitelink`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| invite\_link | String | 是 | 要撤销的邀请链接 |

#### 响应

返回被撤销的 ChatInviteLink 对象。

```
{
  "ok": true,
  "result": {
    "invite_link": "https://t.me/+abc123def456",
    "creator": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "creates_join_request": false,
    "is_primary": false,
    "is_revoked": true,
    "name": "推广链接",
    "expire_date": 1700000000,
    "member_limit": 100,
    "pending_join_request_count": 0
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| invite\_link | String | 邀请链接 URL |
| creator | User | 创建该链接的用户对象 |
| creates\_join\_request | Boolean | 是否需要管理员审批 |
| is\_primary | Boolean | 是否为主邀请链接 |
| is\_revoked | Boolean | 是否已被撤销，撤销后为 `true` |
| name | String | 链接名称 |
| expire\_date | Integer | 过期时间，Unix 时间戳 |
| member\_limit | Integer | 最大成员数限制 |
| pending\_join\_request\_count | Integer | 待审批的加群请求数量 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 invite\_link 格式无效 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权撤销该邀请链接 |
| 404 | 聊天或邀请链接不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-revokechatinvitelink)

---

<a id="page-exportchatinvitelink"></a>

### exportChatInviteLink

导出群组的主邀请链接。每次调用会生成新的主邀请链接，之前的主邀请链接将失效。

#### 请求

`POST /:token/exportchatinvitelink`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |

#### 响应

返回新的邀请链接 URL 字符串。

```
{
  "ok": true,
  "result": "https://t.me/+abc123def456"
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少 chat\_id |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权导出该聊天的邀请链接 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-exportchatinvitelink)

---

<a id="page-approvechatjoinrequest"></a>

### approveChatJoinRequest

批准用户的加群请求。Bot 必须是该聊天的管理员且拥有 can\_invite\_users 权限。

#### 请求

`POST /:token/approvechatjoinrequest`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 要批准的用户唯一标识符 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少必填参数或该用户无待处理的加群请求 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权批准加群请求 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-approvechatjoinrequest)

---

<a id="page-declinechatjoinrequest"></a>

### declineChatJoinRequest

拒绝用户的加群请求。Bot 必须是该聊天的管理员且拥有 can\_invite\_users 权限。

#### 请求

`POST /:token/declinechatjoinrequest`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| user\_id | Integer | 是 | 要拒绝的用户唯一标识符 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如缺少必填参数或该用户无待处理的加群请求 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权拒绝加群请求 |
| 404 | 聊天或用户不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-declinechatjoinrequest)

---

## 其他

<a id="page-answercallbackquery"></a>

### answerCallbackQuery

回复回调查询（内联键盘按钮点击）。收到 callback\_query 后必须调用此方法进行应答，即使不需要向用户显示任何通知。

#### 请求

`POST /:token/answercallbackquery`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| callback\_query\_id | String | 是 | 回调查询的唯一标识符 |
| text | String | 否 | 向用户显示的通知文本，0-200 个字符 |
| show\_alert | Boolean | 否 | 是否以弹窗形式显示通知，默认为 `false` 时显示为顶部浮动提示 |
| url | String | 否 | 用户客户端将要打开的 URL。用于[小游戏](#page-games)时，应返回游戏的 HTTPS 打开地址 |
| cache\_time | Integer | 否 | 回调查询结果的缓存时间（秒），默认为 0 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 callback\_query\_id 无效 |
| 401 | Token 无效或已过期 |
| 403 | 无权回复该回调查询 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-answercallbackquery)

---

<a id="page-answerinlinequery"></a>

### answerInlineQuery

回复内联查询。当用户在任意聊天的输入框中输入 `@bot用户名 关键词` 时，Bot 会通过 [getUpdates](#page-getupdates) 或 Webhook 收到 `inline_query` 更新，需调用本方法返回结果列表供用户选择。

当前版本支持 `article`（文章/文本）和 `game`（游戏）两种结果类型，其他类型会被忽略。

#### 请求

`POST /:token/answerInlineQuery`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| inline\_query\_id | String | 是 | 要应答的内联查询唯一标识符，来自 `inline_query.id` |
| results | InlineQueryResult[] | 是 | 结果对象的 JSON 序列化数组，支持 `article` 和 `game` 两种类型 |
| cache\_time | Integer | 否 | 结果在服务端的缓存时间（秒），默认 300 |

> `is_personal`、`next_offset`、`switch_pm_text`、`switch_pm_parameter` 等 Telegram 官方参数当前版本可以传入但不会生效。

#### 结果类型

##### InlineQueryResultArticle

| 字段 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| type | String | 是 | 固定为 `article` |
| id | String | 是 | 结果的唯一标识符 |
| title | String | 是 | 结果标题 |
| description | String | 否 | 结果的简短描述 |
| input\_message\_content | Object | 是 | 用户选中后要发送的消息内容，当前仅支持 `message_text` 字段（纯文本） |

##### InlineQueryResultGame

| 字段 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| type | String | 是 | 固定为 `game` |
| id | String | 是 | 结果的唯一标识符 |
| game\_short\_name | String | 是 | 游戏的短名称，见[小游戏概览](#page-games) |

> 结果对象中的 `reply_markup` 当前不支持自定义；游戏卡片的默认按钮由服务端自动补齐。

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

用户选中某个结果后，对应消息会以用户的名义发送到该聊天；Bot 可通过 `chosen_inline_result` 更新获知用户的选择。

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 inline\_query\_id 无效、不属于当前 Bot，或 results 不是合法的 JSON 数组 |
| 401 | Token 无效或已过期 |
| 500 | 服务器内部错误 |

#### 示例

##### 返回文章结果

```
curl -X POST "https://api.safew.bot/<token>/answerInlineQuery" \
  -H "Content-Type: application/json" \
  -d '{
    "inline_query_id": "284861963386739430",
    "results": [
      {
        "type": "article",
        "id": "1",
        "title": "今日天气",
        "description": "晴，28°C",
        "input_message_content": {"message_text": "今日天气：晴，28°C"}
      }
    ],
    "cache_time": 60
  }'
```

##### 返回游戏结果

```
curl -X POST "https://api.safew.bot/<token>/answerInlineQuery" \
  -H "Content-Type: application/json" \
  -d '{
    "inline_query_id": "284861963386739430",
    "results": [
      {
        "type": "game",
        "id": "1",
        "game_short_name": "my_game"
      }
    ]
  }'
```

[返回目录](#目录) · [原网页](#page-answerinlinequery)

---

<a id="page-setmycommands"></a>

### setMyCommands

设置 Bot 的命令列表。用户在输入框中输入 `/` 时会看到这些命令。

#### 请求

`POST /:token/setmycommands`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| commands | BotCommand[] | 是 | Bot 命令对象数组，每个对象包含 `command` 和 `description` 字段 |
| scope | BotCommandScope | 否 | 命令的作用范围，默认为所有聊天 |
| language\_code | String | 否 | 语言代码（IETF），用于指定特定语言的命令列表 |

##### BotCommand 对象

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| command | String | 命令文本，1-32 个字符，仅包含小写字母、数字和下划线 |
| description | String | 命令描述，1-256 个字符 |

#### 响应

成功时返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": true
}
```

##### 请求示例

```
{
  "commands": [
    {
      "command": "start",
      "description": "启动机器人"
    },
    {
      "command": "help",
      "description": "获取帮助信息"
    },
    {
      "command": "settings",
      "description": "修改设置"
    }
  ]
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 commands 格式无效或 command 包含非法字符 |
| 401 | Token 无效或已过期 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-setmycommands)

---

<a id="page-getmycommands"></a>

### getMyCommands

获取 Bot 的命令列表。

#### 请求

`POST /:token/getmycommands`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| scope | BotCommandScope | 否 | 命令的作用范围，默认为所有聊天 |
| language\_code | String | 否 | 语言代码（IETF），用于获取特定语言的命令列表 |

#### 响应

返回 BotCommand 对象数组。

```
{
  "ok": true,
  "result": [
    {
      "command": "start",
      "description": "启动机器人"
    },
    {
      "command": "help",
      "description": "获取帮助信息"
    },
    {
      "command": "settings",
      "description": "修改设置"
    }
  ]
}
```

##### BotCommand 字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| command | String | 命令文本 |
| description | String | 命令描述 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误 |
| 401 | Token 无效或已过期 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-getmycommands)

---

<a id="page-getfile"></a>

### getFile

获取文件的下载信息。获取后可通过 `/file/:token/:path/:filename` 下载文件。文件大小不超过 20MB。

#### 请求

`GET /:token/getfile`

`POST /:token/getfile`

本方法同时支持 GET 和 POST 请求。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| file\_id | String | 是 | 要获取信息的文件标识符 |

#### 响应

返回 File 对象。

```
{
  "ok": true,
  "result": {
    "file_id": "BAADBAADAgADr4QLHFZ2Z7RXXXxxx",
    "file_unique_id": "AgADr4QLHFZ2Z7Q",
    "file_size": 1024000,
    "file_path": "documents/file_0.pdf"
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| file\_id | String | 文件的唯一标识符 |
| file\_unique\_id | String | 文件的全局唯一标识符，跨 Bot 不变 |
| file\_size | Integer | 文件大小（字节） |
| file\_path | String | 文件路径，可用于下载文件 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 file\_id 无效 |
| 401 | Token 无效或已过期 |
| 404 | 文件不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-getfile)

---

<a id="page-getuserprofilephotos"></a>

### getUserProfilePhotos

获取用户的头像列表。

#### 请求

`GET /:token/getuserprofilephotos`

`POST /:token/getuserprofilephotos`

本方法同时支持 GET 和 POST 请求。

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| user\_id | Integer | 是 | 目标用户的唯一标识符 |
| offset | Integer | 否 | 返回结果的起始偏移量，默认为 0 |
| limit | Integer | 否 | 返回的头像数量，取值范围 1-100，默认为 100 |

#### 响应

返回 UserProfilePhotos 对象。

```
{
  "ok": true,
  "result": {
    "total_count": 3,
    "photos": [
      [
        {
          "file_id": "AgACAgIAAxkBAAI...",
          "file_unique_id": "AQADAgATxxx",
          "file_size": 8500,
          "width": 160,
          "height": 160
        },
        {
          "file_id": "AgACAgIAAxkBAAI...",
          "file_unique_id": "AQADAgATyyy",
          "file_size": 25000,
          "width": 320,
          "height": 320
        }
      ],
      [
        {
          "file_id": "AgACAgIAAxkBAAI...",
          "file_unique_id": "AQADAgATzzz",
          "file_size": 9200,
          "width": 160,
          "height": 160
        }
      ]
    ]
  }
}
```

##### 返回字段说明

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| total\_count | Integer | 该用户的头像总数 |
| photos | PhotoSize[][] | 头像数组，每个头像包含多个不同尺寸的版本 |

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 limit 超出范围 |
| 401 | Token 无效或已过期 |
| 404 | 用户不存在 |
| 500 | 服务器内部错误 |

[返回目录](#目录) · [原网页](#page-getuserprofilephotos)

---

## 小游戏

<a id="page-games"></a>

### 小游戏（Games）

小游戏让 Bot 在会话中发出一种特殊的「游戏卡片」。用户点击卡片上的按钮后，客户端会在内置 WebView 中打开一个 HTML5 游戏；游戏结束时通过 Bot API 回传分数，并把高分榜直接展示在卡片上。

> 小游戏基于 SafeW 的 **HTML5 Games** 协议，与 [Mini App](https://docs.safew.org/mini-app/) 是两套独立的能力：游戏运行在普通网页中，不依赖 Mini App 的全屏、安全区、主按钮等 WebApp 接口。

#### 准备工作

1. 通过 @BotFather 的 `/newbot` 创建一个 Bot 并获取 Token（详见[快速开始](https://docs.safew.org/guide/quickstart)）。
2. 通过 @BotFather 的 `/newgame` 为该 Bot 注册一个游戏，得到游戏的 `short_name`（见下文）。
3. 把游戏页面部署到一个可公网访问的 HTTPS 地址。

#### 创建游戏（/newgame）

向 @BotFather 发送 `/newgame`，按提示依次录入：

| 步骤 | 说明 |
| --- | --- |
| 选择 Bot | 选择由哪个 Bot 提供该游戏 |
| 标题 Title | 游戏名称 |
| 描述 Description | 游戏的简短介绍 |
| 封面图 Photo | 游戏卡片封面，建议 640×360 |
| 演示动图 GIF | 可选，发送 `/empty` 跳过 |
| 游戏地址 URL | 游戏的 HTTPS 地址 |
| 短名 short\_name | 游戏的唯一标识，3–30 个字符，仅含 `a-z A-Z 0-9 _` |

创建成功后，`short_name` 即可作为 [sendGame](#page-sendgame) 的 `game_short_name` 使用。同一个 Bot 下 `short_name` 必须唯一。

#### 工作流程

```
1. 发送游戏卡片
   Bot ── sendGame{chat_id, game_short_name} ──▶ 会话中出现游戏卡片

2. 用户打开游戏
   用户点击卡片上的游戏按钮
   → Bot 收到一条 callback_query（携带 game_short_name，无 data）
   → Bot 用 answerCallbackQuery{url} 返回游戏地址
   → 客户端在 WebView 中打开该地址

3. 回传分数
   游戏内 ── setGameScore{user_id, score, …} ──▶ 服务端记录分数
   → 默认自动编辑原卡片，刷新高分榜

4. 查看高分榜
   Bot/客户端 ── getGameHighScores{user_id, …} ──▶ 返回高分榜
```

#### 游戏按钮（callback\_game）

游戏卡片上的按钮是一个特殊的内联键盘按钮：它带有 `callback_game` 字段，点击后客户端会发起游戏回调（而不是普通的 `callback_data` 回调）。

- 调用 [sendGame](#page-sendgame) 时，如果不传 `reply_markup`，服务端会自动追加一个「播放 + 游戏标题」按钮。
- 如果自定义 `reply_markup`，**第一行的第一个按钮必须**是 `callback_game` 按钮，用于启动游戏。

```
{
  "inline_keyboard": [
    [{ "text": "▶️ 开始游戏", "callback_game": {} }],
    [{ "text": "玩法说明", "url": "https://example.com/how-to-play" }]
  ]
}
```

`callback_game` 当前是一个占位对象，无需填写任何字段。

#### 打开游戏（处理游戏回调）

当用户点击游戏按钮时，Bot 会通过 [getUpdates](#page-getupdates) 或 Webhook 收到一条 `callback_query`。游戏回调的特征是 `game_short_name` 非空、且不含 `data` 字段，据此可与普通的 `callback_data` 回调区分。

CallbackQuery 的主要字段：

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| id | String | 回调查询的唯一标识符，应答时回传 |
| from | User | 触发回调的用户 |
| message | Message | 携带游戏按钮的消息（消息载体时存在） |
| inline\_message\_id | String | 内联消息标识符（内联载体时存在） |
| data | String | 普通回调数据；游戏回调时不含此字段 |
| game\_short\_name | String | 触发回调的游戏短名；用于识别游戏回调 |

Bot 收到后，应调用 [answerCallbackQuery](#page-answercallbackquery) 并在 `url` 中返回游戏的打开地址：

```
curl -X POST "https://api.safew.bot/<token>/answerCallbackQuery" \
  -H "Content-Type: application/json" \
  -d '{
    "callback_query_id": "<callback_query_id>",
    "url": "https://example.com/game?session=<signed_token>"
  }'
```

建议在 `url` 中附带一次性、带签名和有效期的会话参数（标识 `user_id`、所在会话/消息、过期时间），供游戏前端在调用 [setGameScore](#page-setgamescore) 时定位实例并防止伪造分数。

> 平台也可由服务端直接下发已签名的游戏地址（无需 Bot 往返）。因此 `answerCallbackQuery{url}` 是标准且通用的接入方式，但并非唯一方式。

#### 提交与展示分数

- 游戏结束后，用 [setGameScore](#page-setgamescore) 提交分数。默认情况下，新分数必须**严格大于**该用户的现有分数，否则会被拒绝；可用 `force=true` 强制覆盖。
- 提交成功后默认会自动编辑原游戏卡片，刷新其上的高分榜；传 `disable_edit_message=true` 可关闭此行为。
- 用 [getGameHighScores](#page-getgamehighscores) 拉取高分榜。

#### 数据类型

##### Game

游戏卡片中携带的游戏对象。

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| title | String | 游戏标题 |
| description | String | 游戏描述 |
| photo | PhotoSize[] | 游戏封面图，不同尺寸的数组 |
| text | String | 可选，游戏卡片中的简介文本，0–4096 个字符 |
| text\_entities | MessageEntity[] | 可选，简介文本中的特殊实体（如用户名、URL 等） |
| animation | Animation | 可选，展示在游戏卡片中的演示动图 |

##### GameHighScore

高分榜中的一条记录。

| 字段 | 类型 | 描述 |
| --- | --- | --- |
| position | Integer | 名次（从 1 开始） |
| user | User | 该名次对应的用户 |
| score | Integer | 该用户的分数 |

##### CallbackGame

游戏按钮的占位对象，当前不包含任何字段。

#### 方法一览

| 方法 | 描述 | HTTP 方法 |
| --- | --- | --- |
| [sendGame](#page-sendgame) | 发送游戏卡片 | POST |
| [setGameScore](#page-setgamescore) | 提交/更新用户分数 | POST |
| [getGameHighScores](#page-getgamehighscores) | 获取游戏高分榜 | POST |

相关方法：[answerCallbackQuery](#page-answercallbackquery)（处理游戏回调、返回游戏地址）。

[返回目录](#目录) · [原网页](#page-games)

---

<a id="page-sendgame"></a>

### sendGame

发送游戏卡片到指定聊天。用户可点击卡片上的按钮打开游戏。

#### 请求

`POST /:token/sendGame`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| chat\_id | Integer/String | 是 | 目标聊天的唯一标识符或用户名 |
| game\_short\_name | String | 是 | 游戏的短名，作为游戏的唯一标识，通过 @BotFather 的 `/newgame` 创建 |
| reply\_markup | Object | 否 | 内联键盘标记（InlineKeyboardMarkup）。缺省时自动添加一个「播放 + 游戏标题」按钮；若自定义，第一行第一个按钮必须为 `callback_game` 按钮 |

#### 响应

返回发送成功的 Message 对象，其 `game` 字段为游戏对象。

```
{
  "ok": true,
  "result": {
    "message_id": 100,
    "from": {
      "id": 123456789,
      "is_bot": true,
      "first_name": "MyBot",
      "username": "my_bot"
    },
    "chat": {
      "id": 987654321,
      "type": "private"
    },
    "date": 1700000000,
    "game": {
      "title": "Tower Blocks",
      "description": "堆叠方块，挑战最高分！",
      "photo": [
        { "file_id": "AgACAgIAAx...", "width": 640, "height": 360 }
      ]
    },
    "reply_markup": {
      "inline_keyboard": [
        [{ "text": "▶️ Play Tower Blocks", "callback_game": {} }]
      ]
    }
  }
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如 game\_short\_name 不存在或未注册 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权向该聊天发送消息 |
| 404 | 聊天不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/sendGame" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "game_short_name": "tower_blocks"
  }'
```

##### 自定义按钮

```
curl -X POST "https://api.safew.bot/<token>/sendGame" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": 987654321,
    "game_short_name": "tower_blocks",
    "reply_markup": {
      "inline_keyboard": [
        [{ "text": "▶️ 开始游戏", "callback_game": {} }],
        [{ "text": "玩法说明", "url": "https://example.com/how-to-play" }]
      ]
    }
  }'
```

[返回目录](#目录) · [原网页](#page-sendgame)

---

<a id="page-setgamescore"></a>

### setGameScore

设置用户在游戏中的分数。可作用于消息载体（`chat_id` + `message_id`）或内联消息载体（`inline_message_id`）。

#### 请求

`POST /:token/setGameScore`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| user\_id | Integer | 是 | 目标用户的唯一标识符 |
| score | Integer | 是 | 新的分数，必须为非负整数 |
| force | Boolean | 否 | 是否允许分数下降。默认 `false`，此时新分数必须严格大于现有分数；用于纠错或反作弊时可设为 `true` |
| disable\_edit\_message | Boolean | 否 | 是否不自动编辑游戏卡片以刷新高分榜，默认 `false` |
| chat\_id | Integer | 否 | 当未指定 `inline_message_id` 时必填：游戏卡片所在聊天 |
| message\_id | Integer | 否 | 当未指定 `inline_message_id` 时必填：游戏卡片的消息 ID |
| inline\_message\_id | String | 否 | 当未指定 `chat_id` 和 `message_id` 时必填：内联消息标识符 |

> `chat_id` + `message_id` 与 `inline_message_id` 两组参数二选一。

#### 响应

成功时：

- 若为普通消息载体，返回编辑后的 Message 对象；
- 若为内联消息载体，返回 Boolean 值 `true`。

```
{
  "ok": true,
  "result": {
    "message_id": 100,
    "chat": { "id": 987654321, "type": "private" },
    "date": 1700000000,
    "game": {
      "title": "Tower Blocks",
      "description": "堆叠方块，挑战最高分！",
      "photo": [{ "file_id": "AgACAgIAAx...", "width": 640, "height": 360 }],
      "text": "🏆 1. Alice — 980\n2. Bob — 760"
    }
  }
}
```

当 `force=false` 且新分数不大于该用户现有分数时，请求会被拒绝。

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如新分数不大于现有分数（且 force=false）、载体参数缺失或游戏不存在 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权设置该游戏的分数 |
| 404 | 用户、聊天或消息不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL（消息载体）

```
curl -X POST "https://api.safew.bot/<token>/setGameScore" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 111222333,
    "score": 980,
    "chat_id": 987654321,
    "message_id": 100
  }'
```

##### cURL（内联载体）

```
curl -X POST "https://api.safew.bot/<token>/setGameScore" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 111222333,
    "score": 980,
    "inline_message_id": "AAQAAxkBAAI..."
  }'
```

[返回目录](#目录) · [原网页](#page-setgamescore)

---

<a id="page-getgamehighscores"></a>

### getGameHighScores

获取游戏的高分榜数据。返回目标用户及其相邻名次的若干条记录。

#### 请求

`POST /:token/getGameHighScores`

#### 参数

| 参数 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| user\_id | Integer | 是 | 目标用户的唯一标识符 |
| chat\_id | Integer | 否 | 当未指定 `inline_message_id` 时必填：游戏卡片所在聊天 |
| message\_id | Integer | 否 | 当未指定 `inline_message_id` 时必填：游戏卡片的消息 ID |
| inline\_message\_id | String | 否 | 当未指定 `chat_id` 和 `message_id` 时必填：内联消息标识符 |

> `chat_id` + `message_id` 与 `inline_message_id` 两组参数二选一。

#### 响应

返回 GameHighScore 对象数组。结果包含目标用户及其两侧各若干名相邻用户；若目标用户及其相邻者不在前列，还会附带榜首的若干名用户。

```
{
  "ok": true,
  "result": [
    { "position": 1, "user": { "id": 111, "first_name": "Alice" }, "score": 980 },
    { "position": 2, "user": { "id": 222, "first_name": "Bob" }, "score": 760 },
    { "position": 3, "user": { "id": 333, "first_name": "Carol" }, "score": 540 }
  ]
}
```

#### 错误码

| 错误码 | 描述 |
| --- | --- |
| 400 | 请求参数错误，如载体参数缺失或游戏不存在 |
| 401 | Token 无效或已过期 |
| 403 | Bot 无权查看该游戏的高分榜 |
| 404 | 用户、聊天或消息不存在 |
| 500 | 服务器内部错误 |

#### 示例

##### cURL

```
curl -X POST "https://api.safew.bot/<token>/getGameHighScores" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 111222333,
    "chat_id": 987654321,
    "message_id": 100
  }'
```

[返回目录](#目录) · [原网页](#page-getgamehighscores)

---
