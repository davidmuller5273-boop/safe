package safewbot

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"html"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/domain"
)

// exportGroup is one group in the developer member export.
type exportGroup struct {
	ChatID    string
	Title     string
	LiveCount int // getChatMemberCount; -1 when unavailable
	Members   []domain.BotGroupMember
}

func memberRoleMark(m domain.BotGroupMember) string {
	switch {
	case m.IsCreator:
		return "【群主】"
	case m.IsAdmin:
		return "【管理员】"
	}
	return "成员"
}

func memberRank(m domain.BotGroupMember) int {
	switch {
	case m.IsCreator:
		return 0
	case m.IsAdmin:
		return 1
	}
	return 2
}

// orderMembersForExport puts 群主 first, then 管理员, then everyone else
// (stable, so the stored order is kept inside each class).
func orderMembersForExport(rows []domain.BotGroupMember) []domain.BotGroupMember {
	out := append([]domain.BotGroupMember(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool { return memberRank(out[i]) < memberRank(out[j]) })
	return out
}

func isPrivateGroupRow(g domain.BotGroupSettings) bool {
	t := strings.ToLower(strings.TrimSpace(g.ChatType))
	if t == "private" {
		return true
	}
	return t == "" && !strings.HasPrefix(strings.TrimSpace(g.ChatID), "-")
}

func groupTitle(g exportGroup) string {
	if t := strings.TrimSpace(g.Title); t != "" {
		return t
	}
	return "(无标题)"
}

func liveCountText(n int) string {
	if n < 0 {
		return "未知"
	}
	return strconv.Itoa(n)
}

// buildMembersCSV renders the export as UTF-8 CSV with BOM (Excel friendly).
func buildMembersCSV(groups []exportGroup) []byte {
	var buf bytes.Buffer
	buf.WriteString("\ufeff")
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"说明", "SafeW Bot API 无全量成员列表接口（仅有 getChatAdministrators / getChatMember / getChatMemberCount）。本表=各群管理员（导出时实时拉取）+ 机器人历史记录的全部成员（发言/进退群/chat_member 等）；「群实际人数」来自 getChatMemberCount，与已知成员数可能不一致。", "", "", "", "", "", ""})
	_ = cw.Write([]string{"群名称", "群ID", "群实际人数", "已知成员数", "身份备注", "用户ID", "用户名", "显示名称"})
	for _, g := range groups {
		members := orderMembersForExport(g.Members)
		if len(members) == 0 {
			_ = cw.Write([]string{groupTitle(g), g.ChatID, liveCountText(g.LiveCount), "0", "(暂无成员缓存)", "", "", ""})
			continue
		}
		for _, m := range members {
			uname := strings.TrimSpace(m.Username)
			if uname != "" {
				uname = "@" + uname
			}
			_ = cw.Write([]string{groupTitle(g), g.ChatID, liveCountText(g.LiveCount), strconv.Itoa(len(members)),
				memberRoleMark(m), m.UserID, uname, strings.TrimSpace(m.FirstName)})
		}
	}
	cw.Flush()
	return buf.Bytes()
}

// buildMembersText is the paged-text fallback when sendDocument fails.
func buildMembersText(groups []exportGroup) []string {
	var blocks []string
	for _, g := range groups {
		members := orderMembersForExport(g.Members)
		var b strings.Builder
		fmt.Fprintf(&b, "群：%s (%s) 实际%s人 / 已知%d人", groupTitle(g), g.ChatID, liveCountText(g.LiveCount), len(members))
		for _, m := range members {
			name := strings.TrimSpace(m.FirstName)
			uname := strings.TrimSpace(m.Username)
			if uname != "" {
				uname = " @" + uname
			}
			mark := memberRoleMark(m)
			if mark == "成员" {
				mark = ""
			}
			fmt.Fprintf(&b, "\n%s%s %s%s", mark, m.UserID, name, uname)
		}
		blocks = append(blocks, b.String())
	}
	return chunkText(blocks, "\n\n", lotteryMessageMaxRune)
}

// collectExportGroups gathers every known group (private chats skipped), refreshing
// admins via getChatAdministrators and the real size via getChatMemberCount.
func (w worker) collectExportGroups(ctx context.Context, token string) ([]exportGroup, error) {
	const pageSize = 500
	var groups []exportGroup
	for offset := 0; ; offset += pageSize {
		rows, total, err := w.perms.ListGroups(offset, pageSize)
		if err != nil {
			return nil, err
		}
		for _, g := range rows {
			if isPrivateGroupRow(g) {
				continue
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			chatID := g.ChatID
			if admins, err := w.client.GetChatAdministrators(ctx, token, chatID); err == nil {
				infos := make([]botperm.MemberInfo, 0, len(admins))
				for _, a := range admins {
					infos = append(infos, botperm.MemberInfo{UserID: a.User.IDString(), Username: a.User.Username, FirstName: a.User.FirstName, Status: a.Status})
				}
				_ = w.perms.RefreshAdmins(chatID, infos)
			} else {
				log.Printf("导出成员: getChatAdministrators 失败 chat=%s: %v", chatID, err)
			}
			eg := exportGroup{ChatID: chatID, Title: g.Title, LiveCount: -1}
			if n, err := w.client.GetChatMemberCount(ctx, token, chatID); err == nil {
				eg.LiveCount = n
			}
			for mOffset := 0; ; mOffset += pageSize {
				members, mTotal, err := w.perms.ListMembers(chatID, mOffset, pageSize)
				if err != nil {
					return nil, err
				}
				eg.Members = append(eg.Members, members...)
				if len(members) < pageSize || int64(mOffset+pageSize) >= mTotal {
					break
				}
			}
			groups = append(groups, eg)
		}
		if len(rows) < pageSize || int64(offset+pageSize) >= total {
			break
		}
	}
	return groups, nil
}

// cmdExportAllMembers implements the developer-only /导出所有群成员.
func (w worker) cmdExportAllMembers(ctx context.Context, token, replyChat string) error {
	_ = w.replyPlain(ctx, token, replyChat, "正在导出所有群成员，请稍候…")
	groups, err := w.collectExportGroups(ctx, token)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return w.replyPlain(ctx, token, replyChat, "暂无记录到群（把机器人拉进群或群内有人发言后会出现）")
	}
	known, live := 0, 0
	for _, g := range groups {
		known += len(g.Members)
		if g.LiveCount > 0 {
			live += g.LiveCount
		}
	}
	caption := fmt.Sprintf("所有群成员导出：%d 个群，已知成员 %d 人（群实际合计 %d 人）。\nSafeW API 无法列出全部成员；本文件含管理员（实时）+ 机器人曾记录的成员（发言/进退群等），每群管理员排在最前。", len(groups), known, live)
	name := "群成员导出_" + time.Now().Format("20060102_150405") + ".csv"
	if _, err := w.client.SendDocument(ctx, token, replyChat, name, buildMembersCSV(groups), caption); err == nil {
		return nil
	} else {
		log.Printf("sendDocument 失败，改为分页文本: %v", err)
	}
	if err := w.replyPlain(ctx, token, replyChat, html.EscapeString(caption+"\n（文件发送失败，改为分页文本）")); err != nil {
		return err
	}
	for _, page := range buildMembersText(groups) {
		if err := w.replyPlain(ctx, token, replyChat, html.EscapeString(page)); err != nil {
			return err
		}
	}
	return nil
}
