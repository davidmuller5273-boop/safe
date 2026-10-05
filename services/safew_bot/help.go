package safewbot

import (
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/botperm"
)

// Help sections. Developer-only commands (/devgroups, /devmembers, /导出所有群成员,
// global ads, super-admin management) are intentionally never listed here.
const (
	helpQuery = `【查询】
/开奖 [彩种] — 最新开奖（默认本群订阅的彩种）
/开奖历史 彩种 — 最近100期，可翻页
/开奖订阅 — 本群已订阅的彩种
/pushstatus — 本群推送状态
/whoami — 我的 ID 与角色
/菜单 — 按钮菜单`

	helpLottery = `【开奖播报】
/订阅开奖 [全部|福彩|体彩|六合彩|彩种] — 订阅（默认全部）
/取消订阅开奖 [同上] — 取消（全部=清空本群订阅）`

	helpAds = `【广告】
/广告前 文本 — 设置本群前广告（不带文本=清空）
/广告后 文本 — 设置本群后广告（不带文本=清空）
/广告前后 前广告 | 后广告 — 一次设置前后
/adstatus — 查看本群广告
/resetads group — 清除本群广告，回落全局
/say 文本 — 机器人代发一条消息`

	helpPush = `【推送开关】
/push on — 开启本群推送
/push off — 关闭本群推送
/broadcast 文本 — 发给所有已开推送的群`

	helpChampion = `【冠军】
/开启6码 — 只推冠军6码
/开启7码 — 只推冠军7码
/关闭6码 — 关闭冠军6码
/关闭7码 — 关闭冠军7码`

	helpRunnerUp = `【亚军】
/开启亚军6码 — 只推亚军6码
/开启亚军7码 — 只推亚军7码
/关闭亚军6码 — 关闭亚军6码
/关闭亚军7码 — 关闭亚军7码`

	helpGroupAdmins = `【群管理员】
/addgroupadmin 用户ID [群ID] — 添加群管理员
/removegroupadmin 用户ID [群ID] — 移除群管理员
/listgroupadmins [群ID] — 查看群管理员`
)

// helpForRole returns help text containing only what the caller's role can use.
func helpForRole(role string) string {
	sections := []string{helpQuery}
	title := "📖 使用帮助（普通用户）"
	footer := "彩种：双色球 福彩3D 七乐彩 快乐8 大乐透 排列3 排列5 7星彩 香港六合彩 澳门六合彩 新澳六合彩"
	switch role {
	case botperm.RoleDeveloper, botperm.RoleAdmin:
		title = "📖 使用帮助（" + botperm.RoleLabel(role) + "）"
		sections = append(sections, helpPush, helpChampion, helpRunnerUp, helpLottery, helpAds, helpGroupAdmins)
		footer = "进群默认不推送；开启冠军/亚军任一码会自动开推送，冠军与亚军互不影响。\n" + footer
	case botperm.RoleGroupAdmin:
		title = "📖 使用帮助（群管理员）"
		sections = append(sections, helpLottery, helpAds)
	}
	return title + "\n\n" + strings.Join(sections, "\n\n") + "\n\n" + footer
}
