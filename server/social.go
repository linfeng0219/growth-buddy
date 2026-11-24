package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const friendCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

type ChildSocialSelfDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	FriendCode string `json:"friend_code"`
	QRPayload  string `json:"qr_payload"`
	AvatarIdx  int    `json:"avatar_idx"`
	AvatarURL  string `json:"avatar_url"`
}

type FriendItemDTO struct {
	PeerType      string `json:"peer_type"` // child | parent
	PeerID        string `json:"peer_id"`
	Name          string `json:"name"`
	RoleLabel     string `json:"role_label"`
	GroupName     string `json:"group_name"` // 家庭 | 朋友
	IsFamily      bool   `json:"is_family"`
	AvatarIdx     int    `json:"avatar_idx"`
	AvatarURL     string `json:"avatar_url"`
	PetName       string `json:"pet_name,omitempty"`
	PetLevel      int    `json:"pet_level"`
	UnreadCount   int    `json:"unread_count"`
	LastMessage   string `json:"last_message"`
	LastMessageAt int64  `json:"last_message_at"`
}

type FriendRequestDTO struct {
	ID          string `json:"id"`
	FromChildID string `json:"from_child_id"`
	ToChildID   string `json:"to_child_id"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	AvatarIdx   int    `json:"avatar_idx"`
	AvatarURL   string `json:"avatar_url"`
	PetName     string `json:"pet_name"`
	PetLevel    int    `json:"pet_level"`
	Message     string `json:"message"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
}

type FriendLookupDTO struct {
	ChildID        string `json:"child_id"`
	Name           string `json:"name"`
	Username       string `json:"username"`
	AvatarIdx      int    `json:"avatar_idx"`
	AvatarURL      string `json:"avatar_url"`
	PetName        string `json:"pet_name"`
	PetLevel       int    `json:"pet_level"`
	RelationStatus string `json:"relation_status"` // self | family | friend | pending_outgoing | pending_incoming | none
}

type ParentSocialSelfDTO struct {
	ID         string `json:"id"`
	FamilyID   string `json:"family_id"`
	FamilyName string `json:"family_name"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	FriendCode string `json:"friend_code"`
	QRPayload  string `json:"qr_payload"`
	QRSVG      string `json:"qr_svg"`
	AvatarIdx  int    `json:"avatar_idx"`
	AvatarURL  string `json:"avatar_url"`
}

type ParentFriendRequestDTO struct {
	ID           string `json:"id"`
	FromParentID string `json:"from_parent_id"`
	ToParentID   string `json:"to_parent_id"`
	Name         string `json:"name"`
	Username     string `json:"username"`
	FamilyName   string `json:"family_name"`
	AvatarIdx    int    `json:"avatar_idx"`
	AvatarURL    string `json:"avatar_url"`
	Message      string `json:"message"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"created_at"`
}

type ParentFriendLookupDTO struct {
	ParentID       string `json:"parent_id"`
	Name           string `json:"name"`
	Username       string `json:"username"`
	FamilyName     string `json:"family_name"`
	AvatarIdx      int    `json:"avatar_idx"`
	AvatarURL      string `json:"avatar_url"`
	RelationStatus string `json:"relation_status"` // self | family | friend | pending_outgoing | pending_incoming | none
}

type ParentOversightChildDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AvatarIdx  int    `json:"avatar_idx"`
	AvatarURL  string `json:"avatar_url"`
	UnreadChat int    `json:"unread_chat"`
}

type ParentSocialOverviewDTO struct {
	Self             ParentSocialSelfDTO       `json:"self"`
	Groups           []string                  `json:"groups"`
	Friends          []FriendItemDTO           `json:"friends"`
	ChatGroups       []ChatGroupDTO            `json:"chat_groups"`
	IncomingRequests []ParentFriendRequestDTO  `json:"incoming_requests"`
	OutgoingRequests []ParentFriendRequestDTO  `json:"outgoing_requests"`
	Children         []ParentOversightChildDTO `json:"children"`
}

type ChatGroupMemberDTO struct {
	MemberType string `json:"member_type"`
	MemberID   string `json:"member_id"`
	Name       string `json:"name"`
	RoleLabel  string `json:"role_label"`
	AvatarIdx  int    `json:"avatar_idx"`
	AvatarURL  string `json:"avatar_url"`
	Role       string `json:"role"`
}

type ChatGroupDTO struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	IsFamilyDefault bool   `json:"is_family_default"`
	MemberCount     int    `json:"member_count"`
	MembersPreview  string `json:"members_preview"`
	UnreadCount     int    `json:"unread_count"`
	LastMessage     string `json:"last_message"`
	LastMessageAt   int64  `json:"last_message_at"`
}

type ChildSocialOverviewDTO struct {
	Self             ChildSocialSelfDTO `json:"self"`
	Groups           []string           `json:"groups"`
	Friends          []FriendItemDTO    `json:"friends"`
	ChatGroups       []ChatGroupDTO     `json:"chat_groups"`
	IncomingRequests []FriendRequestDTO `json:"incoming_requests"`
	OutgoingRequests []FriendRequestDTO `json:"outgoing_requests"`
}

type ChatMessageDTO struct {
	ID              string `json:"id"`
	SenderType      string `json:"sender_type"`
	SenderID        string `json:"sender_id"`
	SenderName      string `json:"sender_name,omitempty"`
	SenderAvatarIdx int    `json:"sender_avatar_idx"`
	SenderAvatarURL string `json:"sender_avatar_url,omitempty"`
	ReceiverType    string `json:"receiver_type"`
	ReceiverID      string `json:"receiver_id"`
	IsSelf          bool   `json:"is_self"`
	MsgType         string `json:"msg_type"` // text | voice | image
	Text            string `json:"text"`
	MediaURL        string `json:"media_url,omitempty"`
	DurationSec     int    `json:"duration_sec"`
	IsRead          bool   `json:"is_read"`
	CreatedAt       int64  `json:"created_at"`
}

func genFriendCodeRaw() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i := range b {
		out[i] = friendCodeAlphabet[int(b[i])%len(friendCodeAlphabet)]
	}
	return string(out)
}

func (s *Store) uniqueFriendCodeLocked(tx *Tx) string {
	for i := 0; i < 16; i++ {
		code := genFriendCodeRaw()
		var dummy int
		errC := tx.QueryRow("SELECT 1 FROM children WHERE friend_code = ?", func(r SQLiteRow) error {
			dummy = r.Int(0)
			return nil
		}, code)
		if errC == nil && dummy == 1 {
			continue
		}
		errP := tx.QueryRow("SELECT 1 FROM parents WHERE friend_code = ?", func(r SQLiteRow) error {
			dummy = r.Int(0)
			return nil
		}, code)
		if errP != nil || dummy == 0 {
			return code
		}
	}
	return fmt.Sprintf("GB%X", nowMs()%0xFFFFFF)
}

func buildConvKey(typeA, idA, typeB, idB string) string {
	if typeB == "group" {
		return "group:" + idB
	}
	if typeA == "group" {
		return "group:" + idA
	}
	a := typeA + ":" + idA
	b := typeB + ":" + idB
	if a <= b {
		return a + "|" + b
	}
	return b + "|" + a
}

func (s *Store) lookupActorProfileLocked(tx *Tx, actorType, actorID string) (string, int, string) {
	var name, avURL string
	var avIdx int
	switch actorType {
	case "child":
		_ = tx.QueryRow(
			"SELECT name, avatar_idx, (avatar_data != ''), avatar_updated_at FROM children WHERE id = ?",
			func(r SQLiteRow) error {
				name = r.String(0)
				avIdx = r.Int(1)
				avURL = avatarURL("child", actorID, r.Int(2) != 0, r.Int64(3))
				return nil
			}, actorID,
		)
	case "parent":
		_ = tx.QueryRow(
			"SELECT display_name, avatar_idx, (avatar_data != ''), avatar_updated_at FROM parents WHERE id = ?",
			func(r SQLiteRow) error {
				name = r.String(0)
				avIdx = r.Int(1)
				avURL = avatarURL("parent", actorID, r.Int(2) != 0, r.Int64(3))
				return nil
			}, actorID,
		)
	}
	if name == "" {
		name = "成员"
	}
	return name, avIdx, avURL
}

// syncFamilyGroupLocked 确保每个家庭默认拥有一个以家庭名命名的群聊，并将同家庭全部家长与孩子自动拉入。
func (s *Store) syncFamilyGroupLocked(tx *Tx, familyID string) error {
	if familyID == "" {
		return nil
	}
	var famName string
	if err := tx.QueryRow("SELECT name FROM families WHERE id = ?", func(r SQLiteRow) error {
		famName = strings.TrimSpace(r.String(0))
		return nil
	}, familyID); err != nil {
		return nil
	}
	if famName == "" {
		famName = "我的家庭"
	}
	var groupID, curName string
	err := tx.QueryRow(
		"SELECT id, name FROM chat_groups WHERE family_id = ? AND is_family_default = 1 LIMIT 1",
		func(r SQLiteRow) error {
			groupID = r.String(0)
			curName = r.String(1)
			return nil
		}, familyID,
	)
	now := nowMs()
	if err != nil || groupID == "" {
		groupID = "grp_fam_" + familyID
		if _, exErr := tx.Exec(
			"INSERT OR IGNORE INTO chat_groups (id, name, family_id, is_family_default, creator_type, creator_id, created_at) VALUES (?, ?, ?, 1, 'system', '', ?)",
			groupID, famName, familyID, now,
		); exErr != nil {
			return exErr
		}
	} else if curName != famName {
		_, _ = tx.Exec("UPDATE chat_groups SET name = ? WHERE id = ?", famName, groupID)
	}

	var validKeys []string
	var parentIDs, childIDs []string
	_ = tx.Query("SELECT id FROM parents WHERE family_id = ?", func(r SQLiteRow) error {
		parentIDs = append(parentIDs, r.String(0))
		return nil
	}, familyID)
	_ = tx.Query("SELECT id FROM children WHERE family_id = ?", func(r SQLiteRow) error {
		childIDs = append(childIDs, r.String(0))
		return nil
	}, familyID)

	for _, pid := range parentIDs {
		_, _ = tx.Exec(
			"INSERT OR IGNORE INTO chat_group_members (group_id, member_type, member_id, role, last_read_at, joined_at) VALUES (?, 'parent', ?, 'owner', 0, ?)",
			groupID, pid, now,
		)
		validKeys = append(validKeys, "parent:"+pid)
	}
	for _, cid := range childIDs {
		_, _ = tx.Exec(
			"INSERT OR IGNORE INTO chat_group_members (group_id, member_type, member_id, role, last_read_at, joined_at) VALUES (?, 'child', ?, 'member', 0, ?)",
			groupID, cid, now,
		)
		validKeys = append(validKeys, "child:"+cid)
	}
	if len(validKeys) > 0 {
		validSet := make(map[string]bool, len(validKeys))
		for _, k := range validKeys {
			validSet[k] = true
		}
		type memPair struct{ t, id string }
		var curMembers []memPair
		_ = tx.Query("SELECT member_type, member_id FROM chat_group_members WHERE group_id = ?", func(r SQLiteRow) error {
			curMembers = append(curMembers, memPair{r.String(0), r.String(1)})
			return nil
		}, groupID)
		for _, cm := range curMembers {
			if !validSet[cm.t+":"+cm.id] {
				_, _ = tx.Exec("DELETE FROM chat_group_members WHERE group_id = ? AND member_type = ? AND member_id = ?", groupID, cm.t, cm.id)
			}
		}
	}
	return nil
}

func (s *Store) listActorChatGroupsLocked(tx *Tx, actorType, actorID string) ([]ChatGroupDTO, error) {
	groups := make([]ChatGroupDTO, 0)
	if err := tx.Query(
		`SELECT g.id, g.name, g.is_family_default, gm.last_read_at
		 FROM chat_group_members gm
		 JOIN chat_groups g ON g.id = gm.group_id
		 WHERE gm.member_type = ? AND gm.member_id = ?
		 ORDER BY g.is_family_default DESC, g.created_at DESC`,
		func(r SQLiteRow) error {
			gid := r.String(0)
			groups = append(groups, ChatGroupDTO{
				ID:              gid,
				Name:            r.String(1),
				IsFamilyDefault: r.Int(2) != 0,
				LastMessageAt:   r.Int64(3), // 临时存 last_read_at
			})
			return nil
		}, actorType, actorID,
	); err != nil {
		return nil, err
	}

	for i := range groups {
		lastRead := groups[i].LastMessageAt
		groups[i].LastMessageAt = 0
		gid := groups[i].ID

		var names []string
		type mp struct{ t, id string }
		var mps []mp
		_ = tx.Query("SELECT member_type, member_id FROM chat_group_members WHERE group_id = ? ORDER BY joined_at ASC", func(r SQLiteRow) error {
			mps = append(mps, mp{r.String(0), r.String(1)})
			return nil
		}, gid)
		groups[i].MemberCount = len(mps)
		for idx, m := range mps {
			if idx < 4 {
				nm, _, _ := s.lookupActorProfileLocked(tx, m.t, m.id)
				names = append(names, nm)
			}
		}
		if len(mps) > 4 {
			groups[i].MembersPreview = strings.Join(names, "、") + "等"
		} else {
			groups[i].MembersPreview = strings.Join(names, "、")
		}

		var unread int
		_ = tx.QueryRow(
			`SELECT COUNT(*) FROM chat_messages
			 WHERE conv_key = ? AND created_at > ? AND NOT (sender_type = ? AND sender_id = ?)`,
			func(r SQLiteRow) error {
				unread = r.Int(0)
				return nil
			}, "group:"+gid, lastRead, actorType, actorID,
		)
		groups[i].UnreadCount = unread

		var sType, sID, mType, txt string
		var dur int
		var ts int64
		if err := tx.QueryRow(
			"SELECT sender_type, sender_id, msg_type, text_content, duration_sec, created_at FROM chat_messages WHERE conv_key = ? ORDER BY created_at DESC, rowid DESC LIMIT 1",
			func(r SQLiteRow) error {
				sType = r.String(0)
				sID = r.String(1)
				mType = r.String(2)
				txt = r.String(3)
				dur = r.Int(4)
				ts = r.Int64(5)
				return nil
			}, "group:"+gid,
		); err == nil {
			sName, _, _ := s.lookupActorProfileLocked(tx, sType, sID)
			groups[i].LastMessage = sName + "：" + previewForMessage(mType, txt, dur)
			groups[i].LastMessageAt = ts
		}
	}
	return groups, nil
}

func isParticipantOfConvKey(convKey, viewerType, viewerID string) bool {
	parts := strings.SplitN(convKey, "|", 2)
	if len(parts) != 2 {
		return false
	}
	v := viewerType + ":" + viewerID
	return parts[0] == v || parts[1] == v
}

func parseFriendSearchToken(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "growthbuddy://") {
		if u, err := url.Parse(trimmed); err == nil {
			if c := strings.TrimSpace(u.Query().Get("code")); c != "" {
				return strings.ToUpper(c)
			}
			if u2 := strings.TrimSpace(u.Query().Get("user")); u2 != "" {
				return strings.ToLower(u2)
			}
		}
	}
	return trimmed
}

func previewForMessage(msgType, textContent string, durSec int) string {
	switch msgType {
	case "voice":
		if durSec <= 0 {
			durSec = 1
		}
		return fmt.Sprintf("[语音 %d″]", durSec)
	case "image":
		return "[图片]"
	default:
		return truncateRunes(strings.TrimSpace(textContent), 28)
	}
}

func (s *Store) fillChatSummariesLocked(tx *Tx, viewerType, viewerID string, items []FriendItemDTO) {
	for i := range items {
		convKey := buildConvKey(viewerType, viewerID, items[i].PeerType, items[i].PeerID)
		var unread int
		_ = tx.QueryRow(
			"SELECT COUNT(*) FROM chat_messages WHERE conv_key = ? AND receiver_type = ? AND receiver_id = ? AND is_read = 0",
			func(r SQLiteRow) error {
				unread = r.Int(0)
				return nil
			}, convKey, viewerType, viewerID,
		)
		items[i].UnreadCount = unread

		var msgType, txt string
		var dur int
		var ts int64
		if err := tx.QueryRow(
			"SELECT msg_type, text_content, duration_sec, created_at FROM chat_messages WHERE conv_key = ? ORDER BY created_at DESC, rowid DESC LIMIT 1",
			func(r SQLiteRow) error {
				msgType = r.String(0)
				txt = r.String(1)
				dur = r.Int(2)
				ts = r.Int64(3)
				return nil
			}, convKey,
		); err == nil {
			items[i].LastMessage = previewForMessage(msgType, txt, dur)
			items[i].LastMessageAt = ts
		}
	}
}

func (s *Store) GetChildSocialOverview(familyID, childID string) (*ChildSocialOverviewDTO, error) {
	var out ChildSocialOverviewDTO
	out.Groups = []string{"家庭", "朋友"}
	out.Friends = make([]FriendItemDTO, 0)
	out.ChatGroups = make([]ChatGroupDTO, 0)
	out.IncomingRequests = make([]FriendRequestDTO, 0)
	out.OutgoingRequests = make([]FriendRequestDTO, 0)

	err := s.db.WithLock(func(tx *Tx) error {
		var self ChildSocialSelfDTO
		err := tx.QueryRow(
			"SELECT id, name, username, friend_code, avatar_idx, (avatar_data != ''), avatar_updated_at FROM children WHERE id = ? AND family_id = ?",
			func(r SQLiteRow) error {
				self = ChildSocialSelfDTO{
					ID:         r.String(0),
					Name:       r.String(1),
					Username:   r.String(2),
					FriendCode: r.String(3),
					AvatarIdx:  r.Int(4),
					AvatarURL:  avatarURL("child", r.String(0), r.Int(5) != 0, r.Int64(6)),
				}
				return nil
			}, childID, familyID,
		)
		if err != nil {
			return errors.New("孩子档案不存在")
		}
		if strings.TrimSpace(self.FriendCode) == "" {
			self.FriendCode = s.uniqueFriendCodeLocked(tx)
			_, _ = tx.Exec("UPDATE children SET friend_code = ? WHERE id = ?", self.FriendCode, childID)
		}
		self.QRPayload = "growthbuddy://add-friend?code=" + self.FriendCode
		out.Self = self

		// 1. 同家庭成员自动放入「家庭」分组（同家庭父母 + 同家庭兄弟姐妹）
		if err := tx.Query(
			"SELECT id, display_name, avatar_idx, (avatar_data != ''), avatar_updated_at FROM parents WHERE family_id = ? ORDER BY created_at ASC",
			func(r SQLiteRow) error {
				pid := r.String(0)
				dname := r.String(1)
				out.Friends = append(out.Friends, FriendItemDTO{
					PeerType:  "parent",
					PeerID:    pid,
					Name:      dname,
					RoleLabel: "家长",
					GroupName: "家庭",
					IsFamily:  true,
					AvatarIdx: r.Int(2),
					AvatarURL: avatarURL("parent", pid, r.Int(3) != 0, r.Int64(4)),
				})
				return nil
			}, familyID,
		); err != nil {
			return err
		}

		if err := tx.Query(
			"SELECT id, name, avatar_idx, (avatar_data != ''), avatar_updated_at, pet_name, xp FROM children WHERE family_id = ? AND id != ? ORDER BY created_at ASC",
			func(r SQLiteRow) error {
				cid := r.String(0)
				out.Friends = append(out.Friends, FriendItemDTO{
					PeerType:  "child",
					PeerID:    cid,
					Name:      r.String(1),
					RoleLabel: "家人",
					GroupName: "家庭",
					IsFamily:  true,
					AvatarIdx: r.Int(2),
					AvatarURL: avatarURL("child", cid, r.Int(3) != 0, r.Int64(4)),
					PetName:   r.String(5),
					PetLevel:  LevelForXP(r.Int(6)),
				})
				return nil
			}, familyID, childID,
		); err != nil {
			return err
		}

		// 2. 已同意的外部好友放入「朋友」默认分组
		if err := tx.Query(
			`SELECT c.id, c.name, f.group_name, c.avatar_idx, (c.avatar_data != ''), c.avatar_updated_at, c.pet_name, c.xp
			 FROM friendships f
			 JOIN children c ON c.id = f.friend_child_id
			 WHERE f.child_id = ?
			 ORDER BY f.created_at DESC`,
			func(r SQLiteRow) error {
				cid := r.String(0)
				grp := strings.TrimSpace(r.String(2))
				if grp == "" {
					grp = "朋友"
				}
				out.Friends = append(out.Friends, FriendItemDTO{
					PeerType:  "child",
					PeerID:    cid,
					Name:      r.String(1),
					RoleLabel: "小伙伴",
					GroupName: grp,
					IsFamily:  false,
					AvatarIdx: r.Int(3),
					AvatarURL: avatarURL("child", cid, r.Int(4) != 0, r.Int64(5)),
					PetName:   r.String(6),
					PetLevel:  LevelForXP(r.Int(7)),
				})
				return nil
			}, childID,
		); err != nil {
			return err
		}

		s.fillChatSummariesLocked(tx, "child", childID, out.Friends)

		_ = s.syncFamilyGroupLocked(tx, familyID)
		if chatGroups, gErr := s.listActorChatGroupsLocked(tx, "child", childID); gErr == nil {
			out.ChatGroups = chatGroups
		}

		// 3. 待处理好友申请（收到的 + 发出的）
		if err := tx.Query(
			`SELECT fr.id, fr.from_child_id, fr.to_child_id, c.name, c.username, c.avatar_idx, (c.avatar_data != ''), c.avatar_updated_at, c.pet_name, c.xp, fr.message, fr.status, fr.created_at
			 FROM friend_requests fr
			 JOIN children c ON c.id = fr.from_child_id
			 WHERE fr.to_child_id = ? AND fr.status = 'pending'
			 ORDER BY fr.created_at DESC`,
			func(r SQLiteRow) error {
				cid := r.String(1)
				out.IncomingRequests = append(out.IncomingRequests, FriendRequestDTO{
					ID:          r.String(0),
					FromChildID: cid,
					ToChildID:   r.String(2),
					Name:        r.String(3),
					Username:    r.String(4),
					AvatarIdx:   r.Int(5),
					AvatarURL:   avatarURL("child", cid, r.Int(6) != 0, r.Int64(7)),
					PetName:     r.String(8),
					PetLevel:    LevelForXP(r.Int(9)),
					Message:     r.String(10),
					Status:      r.String(11),
					CreatedAt:   r.Int64(12),
				})
				return nil
			}, childID,
		); err != nil {
			return err
		}

		if err := tx.Query(
			`SELECT fr.id, fr.from_child_id, fr.to_child_id, c.name, c.username, c.avatar_idx, (c.avatar_data != ''), c.avatar_updated_at, c.pet_name, c.xp, fr.message, fr.status, fr.created_at
			 FROM friend_requests fr
			 JOIN children c ON c.id = fr.to_child_id
			 WHERE fr.from_child_id = ? AND fr.status = 'pending'
			 ORDER BY fr.created_at DESC`,
			func(r SQLiteRow) error {
				cid := r.String(2)
				out.OutgoingRequests = append(out.OutgoingRequests, FriendRequestDTO{
					ID:          r.String(0),
					FromChildID: r.String(1),
					ToChildID:   cid,
					Name:        r.String(3),
					Username:    r.String(4),
					AvatarIdx:   r.Int(5),
					AvatarURL:   avatarURL("child", cid, r.Int(6) != 0, r.Int64(7)),
					PetName:     r.String(8),
					PetLevel:    LevelForXP(r.Int(9)),
					Message:     r.String(10),
					Status:      r.String(11),
					CreatedAt:   r.Int64(12),
				})
				return nil
			}, childID,
		); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) ResetChildFriendCode(familyID, childID string) (string, string, error) {
	var code string
	err := s.db.WithLock(func(tx *Tx) error {
		code = s.uniqueFriendCodeLocked(tx)
		affected, err := tx.Exec("UPDATE children SET friend_code = ? WHERE id = ? AND family_id = ?", code, childID, familyID)
		if err != nil {
			return err
		}
		if affected == 0 {
			return errors.New("孩子档案不存在")
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return code, "growthbuddy://add-friend?code=" + code, nil
}

func (s *Store) resolveTargetChildLocked(tx *Tx, rawQuery string) (id, famID, name, username string, avIdx int, hasAv bool, avUpd int64, petName string, xp int, err error) {
	token := parseFriendSearchToken(rawQuery)
	if token == "" {
		err = errors.New("请输入好友账号、分享码或扫描好友二维码")
		return
	}
	upperCode := strings.ToUpper(token)
	lowerUser := strings.ToLower(token)

	err = tx.QueryRow(
		`SELECT id, family_id, name, username, avatar_idx, (avatar_data != ''), avatar_updated_at, pet_name, xp
		 FROM children
		 WHERE id = ? OR friend_code = ? OR (username != '' AND LOWER(username) = ?)
		 LIMIT 1`,
		func(r SQLiteRow) error {
			id = r.String(0)
			famID = r.String(1)
			name = r.String(2)
			username = r.String(3)
			avIdx = r.Int(4)
			hasAv = r.Int(5) != 0
			avUpd = r.Int64(6)
			petName = r.String(7)
			xp = r.Int(8)
			return nil
		}, token, upperCode, lowerUser,
	)
	if err != nil {
		var matchedParent int
		_ = tx.QueryRow(
			"SELECT 1 FROM parents WHERE id = ? OR friend_code = ? OR (username != '' AND LOWER(username) = ?) LIMIT 1",
			func(r SQLiteRow) error {
				matchedParent = r.Int(0)
				return nil
			}, token, upperCode, lowerUser,
		)
		if matchedParent == 1 {
			err = errors.New("安全限制：儿童端仅支持添加同龄小伙伴为好友，不可跨家庭添加家长")
		} else {
			err = errors.New("没有找到对应的孩子账号或分享码，请核对后重试")
		}
	}
	return
}

func (s *Store) LookupFriendTarget(familyID, childID, rawQuery string) (*FriendLookupDTO, error) {
	var out FriendLookupDTO
	err := s.db.WithLock(func(tx *Tx) error {
		tid, tfamID, tname, tuser, avIdx, hasAv, avUpd, petName, xp, err := s.resolveTargetChildLocked(tx, rawQuery)
		if err != nil {
			return err
		}
		rel := "none"
		switch {
		case tid == childID:
			rel = "self"
		case tfamID == familyID:
			rel = "family"
		default:
			var dummy int
			if tx.QueryRow("SELECT 1 FROM friendships WHERE child_id = ? AND friend_child_id = ?", func(r SQLiteRow) error {
				dummy = r.Int(0)
				return nil
			}, childID, tid) == nil && dummy == 1 {
				rel = "friend"
			} else if tx.QueryRow("SELECT 1 FROM friend_requests WHERE from_child_id = ? AND to_child_id = ? AND status = 'pending'", func(r SQLiteRow) error {
				dummy = r.Int(0)
				return nil
			}, childID, tid) == nil && dummy == 1 {
				rel = "pending_outgoing"
			} else if tx.QueryRow("SELECT 1 FROM friend_requests WHERE from_child_id = ? AND to_child_id = ? AND status = 'pending'", func(r SQLiteRow) error {
				dummy = r.Int(0)
				return nil
			}, tid, childID) == nil && dummy == 1 {
				rel = "pending_incoming"
			}
		}

		out = FriendLookupDTO{
			ChildID:        tid,
			Name:           tname,
			Username:       tuser,
			AvatarIdx:      avIdx,
			AvatarURL:      avatarURL("child", tid, hasAv, avUpd),
			PetName:        petName,
			PetLevel:       LevelForXP(xp),
			RelationStatus: rel,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) SendFriendRequest(familyID, fromChildID, rawTarget, greeting string) (string, string, error) {
	cleanMsg := truncateRunes(strings.TrimSpace(greeting), 60)
	if cleanMsg == "" {
		cleanMsg = "想和你做朋友，一起打卡成长呀！"
	}
	var statusOut, noticeOut string

	err := s.db.WithLock(func(tx *Tx) error {
		tid, tfamID, tname, _, _, _, _, _, _, err := s.resolveTargetChildLocked(tx, rawTarget)
		if err != nil {
			return err
		}
		if tid == fromChildID {
			return errors.New("不能添加自己为好友哦")
		}
		if tfamID == familyID {
			return errors.New("你们属于同一个家庭，已经在「家庭」分组自动成为好友啦！")
		}

		var alreadyFriend int
		_ = tx.QueryRow("SELECT 1 FROM friendships WHERE child_id = ? AND friend_child_id = ?", func(r SQLiteRow) error {
			alreadyFriend = r.Int(0)
			return nil
		}, fromChildID, tid)
		if alreadyFriend == 1 {
			return errors.New("你们已经是好友啦！")
		}

		now := nowMs()
		// 若对方已经向我发过待处理申请，直接互相通过并加入「朋友」分组
		var reverseReqID string
		_ = tx.QueryRow(
			"SELECT id FROM friend_requests WHERE from_child_id = ? AND to_child_id = ? AND status = 'pending' LIMIT 1",
			func(r SQLiteRow) error {
				reverseReqID = r.String(0)
				return nil
			}, tid, fromChildID,
		)
		if reverseReqID != "" {
			if _, err := tx.Exec("UPDATE friend_requests SET status = 'accepted', updated_at = ? WHERE id = ?", now, reverseReqID); err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO friendships (child_id, friend_child_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", fromChildID, tid, now); err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO friendships (child_id, friend_child_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", tid, fromChildID, now); err != nil {
				return err
			}
			statusOut = "accepted"
			noticeOut = fmt.Sprintf("对方也发过好友申请，你们已直接成为好友，并放入「朋友」分组！")
			return nil
		}

		// 检查是否已有待处理的我方发出申请，有则刷新时间与留言
		var existingReqID string
		_ = tx.QueryRow(
			"SELECT id FROM friend_requests WHERE from_child_id = ? AND to_child_id = ? AND status = 'pending' LIMIT 1",
			func(r SQLiteRow) error {
				existingReqID = r.String(0)
				return nil
			}, fromChildID, tid,
		)
		if existingReqID != "" {
			_, err := tx.Exec("UPDATE friend_requests SET message = ?, updated_at = ? WHERE id = ?", cleanMsg, now, existingReqID)
			if err != nil {
				return err
			}
			statusOut = "pending"
			noticeOut = fmt.Sprintf("已重新发送好友申请给 %s，等待对方同意", tname)
			return nil
		}

		if _, err := tx.Exec(
			"INSERT INTO friend_requests (id, from_child_id, to_child_id, message, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', ?, ?)",
			GenID("freq_"), fromChildID, tid, cleanMsg, now, now,
		); err != nil {
			return err
		}
		statusOut = "pending"
		noticeOut = fmt.Sprintf("已向 %s 发送好友申请，等待对方同意", tname)
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return statusOut, noticeOut, nil
}

func (s *Store) RespondFriendRequest(childID, reqID, action string) (string, error) {
	act := strings.ToLower(strings.TrimSpace(action))
	if act != "accept" && act != "reject" {
		return "", errors.New("无效的处理动作")
	}
	var notice string
	err := s.db.WithLock(func(tx *Tx) error {
		var fromID, toID, status string
		err := tx.QueryRow(
			"SELECT from_child_id, to_child_id, status FROM friend_requests WHERE id = ?",
			func(r SQLiteRow) error {
				fromID = r.String(0)
				toID = r.String(1)
				status = r.String(2)
				return nil
			}, reqID,
		)
		if err != nil || toID != childID {
			return errors.New("好友申请不存在")
		}
		if status != "pending" {
			return errors.New("该好友申请已处理")
		}
		now := nowMs()
		if act == "reject" {
			if _, err := tx.Exec("UPDATE friend_requests SET status = 'rejected', updated_at = ? WHERE id = ?", now, reqID); err != nil {
				return err
			}
			notice = "已拒绝该好友申请"
			return nil
		}
		if _, err := tx.Exec("UPDATE friend_requests SET status = 'accepted', updated_at = ? WHERE id = ?", now, reqID); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO friendships (child_id, friend_child_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", childID, fromID, now); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO friendships (child_id, friend_child_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", fromID, childID, now); err != nil {
			return err
		}
		notice = "已同意好友申请，对方已加入「朋友」分组！"
		return nil
	})
	if err != nil {
		return "", err
	}
	return notice, nil
}

func (s *Store) RemoveFriend(childID, peerID string) error {
	return s.db.WithLock(func(tx *Tx) error {
		affected, err := tx.Exec(
			"DELETE FROM friendships WHERE (child_id = ? AND friend_child_id = ?) OR (child_id = ? AND friend_child_id = ?)",
			childID, peerID, peerID, childID,
		)
		if err != nil {
			return err
		}
		if affected == 0 {
			return errors.New("未找到该朋友关系（家庭成员无法删除）")
		}
		return nil
	})
}

func (s *Store) lookupActorFamilyLocked(tx *Tx, actorType, actorID string) (string, error) {
	var famID string
	var err error
	switch actorType {
	case "child":
		err = tx.QueryRow("SELECT family_id FROM children WHERE id = ?", func(r SQLiteRow) error {
			famID = r.String(0)
			return nil
		}, actorID)
	case "parent":
		err = tx.QueryRow("SELECT family_id FROM parents WHERE id = ?", func(r SQLiteRow) error {
			famID = r.String(0)
			return nil
		}, actorID)
	default:
		return "", errors.New("无效用户类型")
	}
	if err != nil || famID == "" {
		return "", errors.New("聊天对象不存在")
	}
	return famID, nil
}

func (s *Store) canActorsChatLocked(tx *Tx, typeA, idA, typeB, idB string) (bool, error) {
	if idA == "" || idB == "" || (typeA == typeB && idA == idB) {
		return false, nil
	}
	if typeB == "group" {
		var ok int
		_ = tx.QueryRow(
			"SELECT 1 FROM chat_group_members WHERE group_id = ? AND member_type = ? AND member_id = ?",
			func(r SQLiteRow) error {
				ok = r.Int(0)
				return nil
			}, idB, typeA, idA,
		)
		return ok == 1, nil
	}
	if typeA == "group" {
		var ok int
		_ = tx.QueryRow(
			"SELECT 1 FROM chat_group_members WHERE group_id = ? AND member_type = ? AND member_id = ?",
			func(r SQLiteRow) error {
				ok = r.Int(0)
				return nil
			}, idA, typeB, idB,
		)
		return ok == 1, nil
	}
	famA, err := s.lookupActorFamilyLocked(tx, typeA, idA)
	if err != nil {
		return false, err
	}
	famB, err := s.lookupActorFamilyLocked(tx, typeB, idB)
	if err != nil {
		return false, err
	}
	if famA == famB {
		return true, nil
	}
	if typeA == "child" && typeB == "child" {
		var ok int
		_ = tx.QueryRow("SELECT 1 FROM friendships WHERE child_id = ? AND friend_child_id = ?", func(r SQLiteRow) error {
			ok = r.Int(0)
			return nil
		}, idA, idB)
		return ok == 1, nil
	}
	if typeA == "parent" && typeB == "parent" {
		var ok int
		_ = tx.QueryRow("SELECT 1 FROM parent_friendships WHERE parent_id = ? AND friend_parent_id = ?", func(r SQLiteRow) error {
			ok = r.Int(0)
			return nil
		}, idA, idB)
		return ok == 1, nil
	}
	return false, nil
}

func parseChatMedia(raw, fallbackMime string, allowedPrefix string, maxBytes int) (string, []byte, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil, "", errors.New("媒体内容不能为空")
	}
	mimeType := strings.ToLower(strings.TrimSpace(fallbackMime))
	b64 := trimmed
	if strings.HasPrefix(trimmed, "data:") {
		comma := strings.IndexByte(trimmed, ',')
		if comma > 5 {
			meta := trimmed[5:comma]
			if semi := strings.IndexByte(meta, ';'); semi > 0 {
				mimeType = strings.ToLower(strings.TrimSpace(meta[:semi]))
			} else if meta != "" {
				mimeType = strings.ToLower(strings.TrimSpace(meta))
			}
			b64 = trimmed[comma+1:]
		}
	}
	if mimeType == "" {
		if allowedPrefix == "image/" {
			mimeType = "image/jpeg"
		} else {
			mimeType = "audio/mp4"
		}
	}
	if !strings.HasPrefix(mimeType, allowedPrefix) {
		return "", nil, "", errors.New("不支持的文件格式")
	}
	b64 = strings.TrimSpace(b64)
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(decoded) == 0 {
		return "", nil, "", errors.New("媒体编码格式无效")
	}
	if len(decoded) > maxBytes {
		return "", nil, "", fmt.Errorf("文件体积过大（上限 %d MB）", maxBytes/(1024*1024))
	}
	cleanB64 := base64.StdEncoding.EncodeToString(decoded)
	return mimeType, decoded, cleanB64, nil
}

func (s *Store) fetchChatMessagesLocked(tx *Tx, selfType, selfID, convKey string, afterMs int64, maxRows ...int) ([]ChatMessageDTO, error) {
	list := make([]ChatMessageDTO, 0)
	limit := 120
	if len(maxRows) > 0 {
		limit = maxRows[0]
	}
	var q string
	var args []any
	if afterMs > 0 {
		q = `SELECT id, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at
			 FROM chat_messages
			 WHERE conv_key = ? AND created_at > ?
			 ORDER BY created_at ASC, rowid ASC`
		if limit >= 0 {
			incLim := limit
			if incLim < 200 {
				incLim = 200
			}
			q += fmt.Sprintf(" LIMIT %d", incLim)
		}
		args = []any{convKey, afterMs}
	} else if limit < 0 {
		q = `SELECT id, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at
			 FROM chat_messages
			 WHERE conv_key = ?
			 ORDER BY created_at ASC, rowid ASC`
		args = []any{convKey}
	} else {
		if limit == 0 {
			limit = 120
		}
		q = fmt.Sprintf(`SELECT id, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at
			 FROM (
				SELECT id, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at, rowid AS _rid
				FROM chat_messages
				WHERE conv_key = ?
				ORDER BY created_at DESC, rowid DESC LIMIT %d
			 ) sub
			 ORDER BY created_at ASC, _rid ASC`, limit)
		args = []any{convKey}
	}

	type actorProf struct {
		name   string
		idx    int
		avatar string
	}
	profCache := make(map[string]actorProf)
	getProf := func(sType, sID string) actorProf {
		k := sType + ":" + sID
		if p, ok := profCache[k]; ok {
			return p
		}
		nm, idx, url := s.lookupActorProfileLocked(tx, sType, sID)
		p := actorProf{name: nm, idx: idx, avatar: url}
		profCache[k] = p
		return p
	}

	err := tx.Query(q, func(r SQLiteRow) error {
		sType := r.String(1)
		sID := r.String(2)
		mID := r.String(7)
		mURL := ""
		if mID != "" {
			mURL = "/api/chat/media/" + mID
		}
		prof := getProf(sType, sID)
		list = append(list, ChatMessageDTO{
			ID:              r.String(0),
			SenderType:      sType,
			SenderID:        sID,
			SenderName:      prof.name,
			SenderAvatarIdx: prof.idx,
			SenderAvatarURL: prof.avatar,
			ReceiverType:    r.String(3),
			ReceiverID:      r.String(4),
			IsSelf:          sType == selfType && sID == selfID,
			MsgType:         r.String(5),
			Text:            r.String(6),
			MediaURL:        mURL,
			DurationSec:     r.Int(8),
			IsRead:          r.Int(9) != 0,
			CreatedAt:       r.Int64(10),
		})
		return nil
	}, args...)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) ListChatMessages(viewerType, viewerID, peerType, peerID string, afterMs int64, maxRows ...int) ([]ChatMessageDTO, error) {
	var list []ChatMessageDTO
	convKey := buildConvKey(viewerType, viewerID, peerType, peerID)

	err := s.db.WithLock(func(tx *Tx) error {
		allowed, err := s.canActorsChatLocked(tx, viewerType, viewerID, peerType, peerID)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("你们还不是好友，无法查看聊天记录")
		}

		// 标记对方发给我的消息为已读（群聊更新个人 last_read_at）
		if peerType == "group" {
			_, _ = tx.Exec(
				"UPDATE chat_group_members SET last_read_at = ? WHERE group_id = ? AND member_type = ? AND member_id = ?",
				nowMs(), peerID, viewerType, viewerID,
			)
		} else {
			_, _ = tx.Exec(
				"UPDATE chat_messages SET is_read = 1 WHERE conv_key = ? AND receiver_type = ? AND receiver_id = ? AND is_read = 0",
				convKey, viewerType, viewerID,
			)
		}

		list, err = s.fetchChatMessagesLocked(tx, viewerType, viewerID, convKey, afterMs, maxRows...)
		return err
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) SendChatMessage(senderType, senderID, receiverType, receiverID, msgType, textContent, mediaData, mimeType string, durationSec int) (*ChatMessageDTO, error) {
	mType := strings.ToLower(strings.TrimSpace(msgType))
	if mType != "text" && mType != "voice" && mType != "image" {
		return nil, errors.New("仅支持发送文字、语音或图片消息")
	}
	cleanText := truncateRunes(strings.TrimSpace(textContent), 500)
	var cleanMime, cleanB64 string
	var decodedLen int

	switch mType {
	case "text":
		if cleanText == "" {
			return nil, errors.New("消息内容不能为空")
		}
	case "image":
		var dec []byte
		var err error
		cleanMime, dec, cleanB64, err = parseChatMedia(mediaData, mimeType, "image/", 3*1024*1024)
		if err != nil {
			return nil, err
		}
		decodedLen = len(dec)
		if cleanText == "" {
			cleanText = "[图片]"
		}
	case "voice":
		var dec []byte
		var err error
		cleanMime, dec, cleanB64, err = parseChatMedia(mediaData, mimeType, "audio/", 2*1024*1024)
		if err != nil {
			return nil, err
		}
		decodedLen = len(dec)
		durationSec = clampInt(durationSec, 1, 120)
		cleanText = fmt.Sprintf("[语音 %d″]", durationSec)
	}

	convKey := buildConvKey(senderType, senderID, receiverType, receiverID)
	msgID := GenID("msg_")
	mediaID := ""
	mediaURL := ""
	if cleanB64 != "" {
		mediaID = GenID("cmed_")
		mediaURL = "/api/chat/media/" + mediaID
	}
	now := nowMs()

	var sName, sAvatarURL string
	var sAvatarIdx int
	err := s.db.WithLock(func(tx *Tx) error {
		allowed, err := s.canActorsChatLocked(tx, senderType, senderID, receiverType, receiverID)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("你们还不是好友，无法发送消息")
		}
		sName, sAvatarIdx, sAvatarURL = s.lookupActorProfileLocked(tx, senderType, senderID)
		if mediaID != "" {
			if _, err := tx.Exec(
				"INSERT INTO chat_media (id, conv_key, mime_type, data_b64, byte_size, created_at) VALUES (?, ?, ?, ?, ?, ?)",
				mediaID, convKey, cleanMime, cleanB64, decodedLen, now,
			); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO chat_messages (id, conv_key, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
			msgID, convKey, senderType, senderID, receiverType, receiverID, mType, cleanText, mediaID, durationSec, now,
		); err != nil {
			return err
		}
		if receiverType == "group" {
			_, _ = tx.Exec(
				"UPDATE chat_group_members SET last_read_at = ? WHERE group_id = ? AND member_type = ? AND member_id = ?",
				now, receiverID, senderType, senderID,
			)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ChatMessageDTO{
		ID:              msgID,
		SenderType:      senderType,
		SenderID:        senderID,
		SenderName:      sName,
		SenderAvatarIdx: sAvatarIdx,
		SenderAvatarURL: sAvatarURL,
		ReceiverType:    receiverType,
		ReceiverID:      receiverID,
		IsSelf:          true,
		MsgType:         mType,
		Text:            cleanText,
		MediaURL:        mediaURL,
		DurationSec:     durationSec,
		IsRead:          false,
		CreatedAt:       now,
	}, nil
}

func (s *Store) GetChatMedia(viewerType, viewerID, mediaID string) (string, []byte, error) {
	var convKey, mimeType, dataB64 string
	var allowed bool
	err := s.db.WithLock(func(tx *Tx) error {
		if err := tx.QueryRow(
			"SELECT conv_key, mime_type, data_b64 FROM chat_media WHERE id = ?",
			func(r SQLiteRow) error {
				convKey = r.String(0)
				mimeType = r.String(1)
				dataB64 = r.String(2)
				return nil
			}, mediaID,
		); err != nil {
			return err
		}
		if strings.HasPrefix(convKey, "group:") {
			gid := strings.TrimPrefix(convKey, "group:")
			var ok int
			_ = tx.QueryRow(
				"SELECT 1 FROM chat_group_members WHERE group_id = ? AND member_type = ? AND member_id = ?",
				func(r SQLiteRow) error {
					ok = r.Int(0)
					return nil
				}, gid, viewerType, viewerID,
			)
			allowed = ok == 1
			if !allowed && viewerType == "parent" {
				if famID, fErr := s.lookupActorFamilyLocked(tx, "parent", viewerID); fErr == nil {
					var ovOk int
					_ = tx.QueryRow(
						`SELECT 1 FROM chat_group_members gm
						 JOIN children c ON gm.member_type = 'child' AND gm.member_id = c.id
						 WHERE gm.group_id = ? AND c.family_id = ? LIMIT 1`,
						func(r SQLiteRow) error {
							ovOk = r.Int(0)
							return nil
						}, gid, famID,
					)
					allowed = ovOk == 1
				}
			}
		} else {
			allowed = isParticipantOfConvKey(convKey, viewerType, viewerID)
			if !allowed && viewerType == "parent" {
				if famID, fErr := s.lookupActorFamilyLocked(tx, "parent", viewerID); fErr == nil {
					parts := strings.SplitN(convKey, "|", 2)
					for _, pt := range parts {
						if strings.HasPrefix(pt, "child:") {
							cid := strings.TrimPrefix(pt, "child:")
							var ovOk int
							_ = tx.QueryRow(
								"SELECT 1 FROM children WHERE id = ? AND family_id = ?",
								func(r SQLiteRow) error {
									ovOk = r.Int(0)
									return nil
								}, cid, famID,
							)
							if ovOk == 1 {
								allowed = true
								break
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil || dataB64 == "" {
		return "", nil, ErrNotFound
	}
	if !allowed {
		return "", nil, errors.New("无权访问该聊天媒体")
	}
	raw, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return "", nil, ErrNotFound
	}
	return mimeType, raw, nil
}

func (s *Store) buildParentFriendsListLocked(tx *Tx, familyID, parentID string) ([]FriendItemDTO, error) {
	items := make([]FriendItemDTO, 0)
	// 1. 同家庭其他家长（爸爸/妈妈等）放入「家庭」分组
	if err := tx.Query(
		"SELECT id, display_name, avatar_idx, (avatar_data != ''), avatar_updated_at FROM parents WHERE family_id = ? AND id != ? ORDER BY created_at ASC",
		func(r SQLiteRow) error {
			pid := r.String(0)
			items = append(items, FriendItemDTO{
				PeerType:  "parent",
				PeerID:    pid,
				Name:      r.String(1),
				RoleLabel: "家人(家长)",
				GroupName: "家庭",
				IsFamily:  true,
				AvatarIdx: r.Int(2),
				AvatarURL: avatarURL("parent", pid, r.Int(3) != 0, r.Int64(4)),
			})
			return nil
		}, familyID, parentID,
	); err != nil {
		return nil, err
	}
	// 2. 同家庭孩子放入「家庭」分组
	if err := tx.Query(
		"SELECT id, name, avatar_idx, (avatar_data != ''), avatar_updated_at, pet_name, xp FROM children WHERE family_id = ? ORDER BY created_at ASC",
		func(r SQLiteRow) error {
			cid := r.String(0)
			items = append(items, FriendItemDTO{
				PeerType:  "child",
				PeerID:    cid,
				Name:      r.String(1),
				RoleLabel: "孩子",
				GroupName: "家庭",
				IsFamily:  true,
				AvatarIdx: r.Int(2),
				AvatarURL: avatarURL("child", cid, r.Int(3) != 0, r.Int64(4)),
				PetName:   r.String(5),
				PetLevel:  LevelForXP(r.Int(6)),
			})
			return nil
		}, familyID,
	); err != nil {
		return nil, err
	}
	// 3. 已同意的外部家长好友放入「朋友」分组（安全边界：家长只加家长）
	if err := tx.Query(
		`SELECT p.id, p.display_name, pf.group_name, p.avatar_idx, (p.avatar_data != ''), p.avatar_updated_at, fm.name
		 FROM parent_friendships pf
		 JOIN parents p ON p.id = pf.friend_parent_id
		 LEFT JOIN families fm ON fm.id = p.family_id
		 WHERE pf.parent_id = ?
		 ORDER BY pf.created_at DESC`,
		func(r SQLiteRow) error {
			pid := r.String(0)
			grp := strings.TrimSpace(r.String(2))
			if grp == "" {
				grp = "朋友"
			}
			famName := strings.TrimSpace(r.String(6))
			roleLabel := "家长好友"
			if famName != "" {
				roleLabel = famName
			}
			items = append(items, FriendItemDTO{
				PeerType:  "parent",
				PeerID:    pid,
				Name:      r.String(1),
				RoleLabel: roleLabel,
				GroupName: grp,
				IsFamily:  false,
				AvatarIdx: r.Int(3),
				AvatarURL: avatarURL("parent", pid, r.Int(4) != 0, r.Int64(5)),
			})
			return nil
		}, parentID,
	); err != nil {
		return nil, err
	}
	s.fillChatSummariesLocked(tx, "parent", parentID, items)
	return items, nil
}

func (s *Store) ListParentChatContacts(familyID, parentID string) ([]FriendItemDTO, []ChatGroupDTO, error) {
	var items []FriendItemDTO
	groups := make([]ChatGroupDTO, 0)
	err := s.db.WithLock(func(tx *Tx) error {
		var err error
		items, err = s.buildParentFriendsListLocked(tx, familyID, parentID)
		if err != nil {
			return err
		}
		_ = s.syncFamilyGroupLocked(tx, familyID)
		if gList, gErr := s.listActorChatGroupsLocked(tx, "parent", parentID); gErr == nil {
			groups = gList
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return items, groups, nil
}

type GroupMemberRef struct {
	MemberType string `json:"member_type"`
	MemberID   string `json:"member_id"`
}

type ChatGroupDetailDTO struct {
	ChatGroupDTO
	Members []ChatGroupMemberDTO `json:"members"`
}

func (s *Store) getGroupDetailLocked(tx *Tx, groupID, viewerType, viewerID string) (*ChatGroupDetailDTO, error) {
	var dto ChatGroupDetailDTO
	var famID string
	err := tx.QueryRow(
		"SELECT id, name, family_id, is_family_default FROM chat_groups WHERE id = ?",
		func(r SQLiteRow) error {
			dto.ID = r.String(0)
			dto.Name = r.String(1)
			famID = r.String(2)
			dto.IsFamilyDefault = r.Int(3) != 0
			return nil
		}, groupID,
	)
	if err != nil || dto.ID == "" {
		return nil, ErrNotFound
	}
	if dto.IsFamilyDefault && famID != "" {
		_ = s.syncFamilyGroupLocked(tx, famID)
		_ = tx.QueryRow("SELECT name FROM chat_groups WHERE id = ?", func(r SQLiteRow) error {
			dto.Name = r.String(0)
			return nil
		}, groupID)
	}

	var viewerMember int
	_ = tx.QueryRow(
		"SELECT 1 FROM chat_group_members WHERE group_id = ? AND member_type = ? AND member_id = ?",
		func(r SQLiteRow) error {
			viewerMember = r.Int(0)
			return nil
		}, groupID, viewerType, viewerID,
	)
	if viewerMember != 1 {
		return nil, errors.New("你不是该群成员")
	}

	viewerFam, _ := s.lookupActorFamilyLocked(tx, viewerType, viewerID)
	dto.Members = make([]ChatGroupMemberDTO, 0)
	type rawMem struct {
		mType, mID, role string
	}
	var raws []rawMem
	if err := tx.Query(
		"SELECT member_type, member_id, role FROM chat_group_members WHERE group_id = ? ORDER BY CASE WHEN role = 'owner' THEN 0 ELSE 1 END, joined_at ASC",
		func(r SQLiteRow) error {
			raws = append(raws, rawMem{r.String(0), r.String(1), r.String(2)})
			return nil
		}, groupID,
	); err != nil {
		return nil, err
	}

	var names []string
	for idx, rm := range raws {
		nm, aIdx, aURL := s.lookupActorProfileLocked(tx, rm.mType, rm.mID)
		roleLabel := "小伙伴"
		if rm.mType == "parent" {
			roleLabel = "家长"
		} else {
			mFam, _ := s.lookupActorFamilyLocked(tx, rm.mType, rm.mID)
			if viewerFam != "" && mFam == viewerFam {
				roleLabel = "家人"
			}
		}
		dto.Members = append(dto.Members, ChatGroupMemberDTO{
			MemberType: rm.mType,
			MemberID:   rm.mID,
			Name:       nm,
			RoleLabel:  roleLabel,
			AvatarIdx:  aIdx,
			AvatarURL:  aURL,
			Role:       rm.role,
		})
		if idx < 4 {
			names = append(names, nm)
		}
	}
	dto.MemberCount = len(dto.Members)
	if len(dto.Members) > 4 {
		dto.MembersPreview = strings.Join(names, "、") + "等"
	} else {
		dto.MembersPreview = strings.Join(names, "、")
	}
	return &dto, nil
}

func (s *Store) validateCustomGroupSafetyLocked(tx *Tx, members []GroupMemberRef) error {
	parentFams := make(map[string]bool)
	childFams := make(map[string]bool)
	for _, m := range members {
		fam, err := s.lookupActorFamilyLocked(tx, m.MemberType, m.MemberID)
		if err != nil {
			continue
		}
		if m.MemberType == "parent" {
			parentFams[fam] = true
		} else if m.MemberType == "child" {
			childFams[fam] = true
		}
	}
	if len(parentFams) > 0 && len(childFams) > 0 {
		for cf := range childFams {
			for pf := range parentFams {
				if cf != pf {
					return errors.New("安全限制：跨家庭群聊不可同时包含家长与非同家庭孩子")
				}
			}
		}
	}
	return nil
}

func (s *Store) CreateGroupForActor(familyID, creatorType, creatorID, name string, members []GroupMemberRef) (*ChatGroupDetailDTO, error) {
	cleanName := truncateRunes(strings.TrimSpace(name), 30)
	if cleanName == "" {
		return nil, errors.New("请输入群名称")
	}
	if creatorType != "child" && creatorType != "parent" {
		return nil, errors.New("无效创建者类型")
	}
	groupID := GenID("grp_")
	now := nowMs()
	var detail *ChatGroupDetailDTO

	err := s.db.WithLock(func(tx *Tx) error {
		validMembers := []GroupMemberRef{{MemberType: creatorType, MemberID: creatorID}}
		seen := map[string]bool{creatorType + ":" + creatorID: true}
		for _, m := range members {
			mt := strings.TrimSpace(m.MemberType)
			mi := strings.TrimSpace(m.MemberID)
			if (mt != "child" && mt != "parent") || mi == "" {
				continue
			}
			key := mt + ":" + mi
			if seen[key] {
				continue
			}
			ok, err := s.canActorsChatLocked(tx, creatorType, creatorID, mt, mi)
			if err != nil || !ok {
				return errors.New("只能邀请好友或同家庭成员入群")
			}
			seen[key] = true
			validMembers = append(validMembers, GroupMemberRef{MemberType: mt, MemberID: mi})
		}
		if len(validMembers) < 2 {
			return errors.New("请至少选择一位好友或家人一起建群")
		}
		if err := s.validateCustomGroupSafetyLocked(tx, validMembers); err != nil {
			return err
		}
		if _, err := tx.Exec(
			"INSERT INTO chat_groups (id, name, family_id, is_family_default, creator_type, creator_id, created_at) VALUES (?, ?, ?, 0, ?, ?, ?)",
			groupID, cleanName, familyID, creatorType, creatorID, now,
		); err != nil {
			return err
		}
		for idx, vm := range validMembers {
			role := "member"
			if idx == 0 {
				role = "owner"
			}
			if _, err := tx.Exec(
				"INSERT INTO chat_group_members (group_id, member_type, member_id, role, last_read_at, joined_at) VALUES (?, ?, ?, ?, ?, ?)",
				groupID, vm.MemberType, vm.MemberID, role, now, now+int64(idx),
			); err != nil {
				return err
			}
		}
		var err error
		detail, err = s.getGroupDetailLocked(tx, groupID, creatorType, creatorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *Store) CreateFriendGroup(familyID, creatorChildID, name string, members []GroupMemberRef) (*ChatGroupDetailDTO, error) {
	return s.CreateGroupForActor(familyID, "child", creatorChildID, name, members)
}

func (s *Store) GetGroupDetail(groupID, viewerType, viewerID string) (*ChatGroupDetailDTO, error) {
	var detail *ChatGroupDetailDTO
	err := s.db.WithLock(func(tx *Tx) error {
		var err error
		detail, err = s.getGroupDetailLocked(tx, groupID, viewerType, viewerID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *Store) InviteGroupMembers(groupID, actorType, actorID string, members []GroupMemberRef) (*ChatGroupDetailDTO, error) {
	var detail *ChatGroupDetailDTO
	now := nowMs()
	err := s.db.WithLock(func(tx *Tx) error {
		var isFam int
		var exists int
		_ = tx.QueryRow("SELECT 1, is_family_default FROM chat_groups WHERE id = ?", func(r SQLiteRow) error {
			exists = r.Int(0)
			isFam = r.Int(1)
			return nil
		}, groupID)
		if exists != 1 {
			return ErrNotFound
		}
		if isFam != 0 {
			return errors.New("家庭默认群会自动同步家庭成员，无需手动邀请")
		}
		var isMem int
		_ = tx.QueryRow("SELECT 1 FROM chat_group_members WHERE group_id = ? AND member_type = ? AND member_id = ?", func(r SQLiteRow) error {
			isMem = r.Int(0)
			return nil
		}, groupID, actorType, actorID)
		if isMem != 1 {
			return errors.New("只有群成员才能邀请好友入群")
		}

		var existingMembers []GroupMemberRef
		_ = tx.Query("SELECT member_type, member_id FROM chat_group_members WHERE group_id = ?", func(r SQLiteRow) error {
			existingMembers = append(existingMembers, GroupMemberRef{MemberType: r.String(0), MemberID: r.String(1)})
			return nil
		}, groupID)
		var toInsert []GroupMemberRef
		for _, m := range members {
			mt := strings.TrimSpace(m.MemberType)
			mi := strings.TrimSpace(m.MemberID)
			if (mt != "child" && mt != "parent") || mi == "" {
				continue
			}
			ok, err := s.canActorsChatLocked(tx, actorType, actorID, mt, mi)
			if err != nil || !ok {
				return errors.New("只能邀请你的好友或家人入群")
			}
			toInsert = append(toInsert, GroupMemberRef{MemberType: mt, MemberID: mi})
		}
		if err := s.validateCustomGroupSafetyLocked(tx, append(existingMembers, toInsert...)); err != nil {
			return err
		}

		added := 0
		for _, m := range toInsert {
			aff, err := tx.Exec(
				"INSERT OR IGNORE INTO chat_group_members (group_id, member_type, member_id, role, last_read_at, joined_at) VALUES (?, ?, ?, 'member', ?, ?)",
				groupID, m.MemberType, m.MemberID, now, now+int64(added),
			)
			if err != nil {
				return err
			}
			if aff > 0 {
				added++
			}
		}
		if added == 0 {
			return errors.New("所选好友均已在群聊中")
		}
		var err error
		detail, err = s.getGroupDetailLocked(tx, groupID, actorType, actorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *Store) LeaveGroup(groupID, actorType, actorID string) error {
	return s.db.WithLock(func(tx *Tx) error {
		var isFam int
		var exists int
		_ = tx.QueryRow("SELECT 1, is_family_default FROM chat_groups WHERE id = ?", func(r SQLiteRow) error {
			exists = r.Int(0)
			isFam = r.Int(1)
			return nil
		}, groupID)
		if exists != 1 {
			return ErrNotFound
		}
		if isFam != 0 {
			return errors.New("家庭默认群不支持退出")
		}
		aff, err := tx.Exec(
			"DELETE FROM chat_group_members WHERE group_id = ? AND member_type = ? AND member_id = ?",
			groupID, actorType, actorID,
		)
		if err != nil {
			return err
		}
		if aff == 0 {
			return errors.New("你已不在该群聊中")
		}
		var remain int
		_ = tx.QueryRow("SELECT COUNT(*) FROM chat_group_members WHERE group_id = ?", func(r SQLiteRow) error {
			remain = r.Int(0)
			return nil
		}, groupID)
		if remain == 0 {
			_, _ = tx.Exec("DELETE FROM chat_groups WHERE id = ?", groupID)
		}
		return nil
	})
}

/* ==================== 家长端社交（家长互加好友 / 家庭群） & 监护查看儿童好友与聊天 ==================== */

func (s *Store) GetParentSocialOverview(familyID, parentID string) (*ParentSocialOverviewDTO, error) {
	var out ParentSocialOverviewDTO
	out.Groups = []string{"家庭", "朋友"}
	out.Friends = make([]FriendItemDTO, 0)
	out.ChatGroups = make([]ChatGroupDTO, 0)
	out.IncomingRequests = make([]ParentFriendRequestDTO, 0)
	out.OutgoingRequests = make([]ParentFriendRequestDTO, 0)
	out.Children = make([]ParentOversightChildDTO, 0)

	err := s.db.WithLock(func(tx *Tx) error {
		var self ParentSocialSelfDTO
		err := tx.QueryRow(
			`SELECT p.id, p.family_id, COALESCE(fm.name, '我的家庭'), p.display_name, p.username, p.friend_code, p.avatar_idx, (p.avatar_data != ''), p.avatar_updated_at
			 FROM parents p
			 LEFT JOIN families fm ON fm.id = p.family_id
			 WHERE p.id = ? AND p.family_id = ?`,
			func(r SQLiteRow) error {
				self = ParentSocialSelfDTO{
					ID:         r.String(0),
					FamilyID:   r.String(1),
					FamilyName: r.String(2),
					Name:       r.String(3),
					Username:   r.String(4),
					FriendCode: r.String(5),
					AvatarIdx:  r.Int(6),
					AvatarURL:  avatarURL("parent", r.String(0), r.Int(7) != 0, r.Int64(8)),
				}
				return nil
			}, parentID, familyID,
		)
		if err != nil {
			return errors.New("家长账号不存在")
		}
		if strings.TrimSpace(self.FriendCode) == "" {
			self.FriendCode = s.uniqueFriendCodeLocked(tx)
			_, _ = tx.Exec("UPDATE parents SET friend_code = ? WHERE id = ?", self.FriendCode, parentID)
		}
		self.QRPayload = "growthbuddy://add-parent?code=" + self.FriendCode
		self.QRSVG = RenderQRSvg(self.QRPayload, 200)
		out.Self = self

		friends, err := s.buildParentFriendsListLocked(tx, familyID, parentID)
		if err != nil {
			return err
		}
		out.Friends = friends

		_ = s.syncFamilyGroupLocked(tx, familyID)
		if gList, gErr := s.listActorChatGroupsLocked(tx, "parent", parentID); gErr == nil {
			out.ChatGroups = gList
		}

		// 收到的家长好友申请
		if err := tx.Query(
			`SELECT r.id, r.from_parent_id, r.to_parent_id, p.display_name, p.username, COALESCE(fm.name, ''), p.avatar_idx, (p.avatar_data != ''), p.avatar_updated_at, r.message, r.status, r.created_at
			 FROM parent_friend_requests r
			 JOIN parents p ON p.id = r.from_parent_id
			 LEFT JOIN families fm ON fm.id = p.family_id
			 WHERE r.to_parent_id = ? AND r.status = 'pending'
			 ORDER BY r.created_at DESC`,
			func(r SQLiteRow) error {
				pid := r.String(1)
				out.IncomingRequests = append(out.IncomingRequests, ParentFriendRequestDTO{
					ID:           r.String(0),
					FromParentID: pid,
					ToParentID:   r.String(2),
					Name:         r.String(3),
					Username:     r.String(4),
					FamilyName:   r.String(5),
					AvatarIdx:    r.Int(6),
					AvatarURL:    avatarURL("parent", pid, r.Int(7) != 0, r.Int64(8)),
					Message:      r.String(9),
					Status:       r.String(10),
					CreatedAt:    r.Int64(11),
				})
				return nil
			}, parentID,
		); err != nil {
			return err
		}

		// 发出的家长好友申请
		if err := tx.Query(
			`SELECT r.id, r.from_parent_id, r.to_parent_id, p.display_name, p.username, COALESCE(fm.name, ''), p.avatar_idx, (p.avatar_data != ''), p.avatar_updated_at, r.message, r.status, r.created_at
			 FROM parent_friend_requests r
			 JOIN parents p ON p.id = r.to_parent_id
			 LEFT JOIN families fm ON fm.id = p.family_id
			 WHERE r.from_parent_id = ? AND r.status = 'pending'
			 ORDER BY r.created_at DESC`,
			func(r SQLiteRow) error {
				pid := r.String(2)
				out.OutgoingRequests = append(out.OutgoingRequests, ParentFriendRequestDTO{
					ID:           r.String(0),
					FromParentID: r.String(1),
					ToParentID:   pid,
					Name:         r.String(3),
					Username:     r.String(4),
					FamilyName:   r.String(5),
					AvatarIdx:    r.Int(6),
					AvatarURL:    avatarURL("parent", pid, r.Int(7) != 0, r.Int64(8)),
					Message:      r.String(9),
					Status:       r.String(10),
					CreatedAt:    r.Int64(11),
				})
				return nil
			}, parentID,
		); err != nil {
			return err
		}

		// 本家庭孩子摘要（用于家长端查看儿童端好友与聊天记录）
		if err := tx.Query(
			"SELECT id, name, avatar_idx, (avatar_data != ''), avatar_updated_at FROM children WHERE family_id = ? ORDER BY created_at ASC",
			func(r SQLiteRow) error {
				cid := r.String(0)
				out.Children = append(out.Children, ParentOversightChildDTO{
					ID:        cid,
					Name:      r.String(1),
					AvatarIdx: r.Int(2),
					AvatarURL: avatarURL("child", cid, r.Int(3) != 0, r.Int64(4)),
				})
				return nil
			}, familyID,
		); err != nil {
			return err
		}
		for i := range out.Children {
			cid := out.Children[i].ID
			var unread int
			_ = tx.QueryRow("SELECT COUNT(*) FROM chat_messages WHERE receiver_type = 'child' AND receiver_id = ? AND is_read = 0", func(r SQLiteRow) error {
				unread = r.Int(0)
				return nil
			}, cid)
			out.Children[i].UnreadChat = unread
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) ResetParentFriendCode(familyID, parentID string) (string, string, error) {
	var code string
	err := s.db.WithLock(func(tx *Tx) error {
		code = s.uniqueFriendCodeLocked(tx)
		affected, err := tx.Exec("UPDATE parents SET friend_code = ? WHERE id = ? AND family_id = ?", code, parentID, familyID)
		if err != nil {
			return err
		}
		if affected == 0 {
			return errors.New("家长账号不存在")
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return code, "growthbuddy://add-parent?code=" + code, nil
}

func (s *Store) resolveTargetParentLocked(tx *Tx, rawQuery string) (id, famID, name, username, famName string, avIdx int, hasAv bool, avUpd int64, err error) {
	token := parseFriendSearchToken(rawQuery)
	if token == "" {
		err = errors.New("请输入家长账号、分享码或扫描家长名片二维码")
		return
	}
	upperCode := strings.ToUpper(token)
	lowerUser := strings.ToLower(token)

	err = tx.QueryRow(
		`SELECT p.id, p.family_id, p.display_name, p.username, COALESCE(fm.name, ''), p.avatar_idx, (p.avatar_data != ''), p.avatar_updated_at
		 FROM parents p
		 LEFT JOIN families fm ON fm.id = p.family_id
		 WHERE p.id = ? OR p.friend_code = ? OR (p.username != '' AND LOWER(p.username) = ?)
		 LIMIT 1`,
		func(r SQLiteRow) error {
			id = r.String(0)
			famID = r.String(1)
			name = r.String(2)
			username = r.String(3)
			famName = r.String(4)
			avIdx = r.Int(5)
			hasAv = r.Int(6) != 0
			avUpd = r.Int64(7)
			return nil
		}, token, upperCode, lowerUser,
	)
	if err != nil {
		var matchedChild int
		_ = tx.QueryRow(
			"SELECT 1 FROM children WHERE id = ? OR friend_code = ? OR (username != '' AND LOWER(username) = ?) LIMIT 1",
			func(r SQLiteRow) error {
				matchedChild = r.Int(0)
				return nil
			}, token, upperCode, lowerUser,
		)
		if matchedChild == 1 {
			err = errors.New("安全限制：家长端仅支持添加其他家长为好友，不可跨家庭添加孩子")
		} else {
			err = errors.New("没有找到对应的家长账号或分享码，请核对后重试")
		}
	}
	return
}

func (s *Store) LookupParentFriendTarget(familyID, parentID, rawQuery string) (*ParentFriendLookupDTO, error) {
	var out ParentFriendLookupDTO
	err := s.db.WithLock(func(tx *Tx) error {
		tid, tfamID, tname, tuser, famName, avIdx, hasAv, avUpd, err := s.resolveTargetParentLocked(tx, rawQuery)
		if err != nil {
			return err
		}
		rel := "none"
		switch {
		case tid == parentID:
			rel = "self"
		case tfamID == familyID:
			rel = "family"
		default:
			var dummy int
			if tx.QueryRow("SELECT 1 FROM parent_friendships WHERE parent_id = ? AND friend_parent_id = ?", func(r SQLiteRow) error {
				dummy = r.Int(0)
				return nil
			}, parentID, tid) == nil && dummy == 1 {
				rel = "friend"
			} else if tx.QueryRow("SELECT 1 FROM parent_friend_requests WHERE from_parent_id = ? AND to_parent_id = ? AND status = 'pending'", func(r SQLiteRow) error {
				dummy = r.Int(0)
				return nil
			}, parentID, tid) == nil && dummy == 1 {
				rel = "pending_outgoing"
			} else if tx.QueryRow("SELECT 1 FROM parent_friend_requests WHERE from_parent_id = ? AND to_parent_id = ? AND status = 'pending'", func(r SQLiteRow) error {
				dummy = r.Int(0)
				return nil
			}, tid, parentID) == nil && dummy == 1 {
				rel = "pending_incoming"
			}
		}

		out = ParentFriendLookupDTO{
			ParentID:       tid,
			Name:           tname,
			Username:       tuser,
			FamilyName:     famName,
			AvatarIdx:      avIdx,
			AvatarURL:      avatarURL("parent", tid, hasAv, avUpd),
			RelationStatus: rel,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) SendParentFriendRequest(familyID, fromParentID, rawTarget, greeting string) (string, string, error) {
	cleanMsg := truncateRunes(strings.TrimSpace(greeting), 60)
	if cleanMsg == "" {
		cleanMsg = "您好，想与您加为家长好友方便沟通！"
	}
	var statusOut, noticeOut string

	err := s.db.WithLock(func(tx *Tx) error {
		tid, tfamID, tname, _, _, _, _, _, err := s.resolveTargetParentLocked(tx, rawTarget)
		if err != nil {
			return err
		}
		if tid == fromParentID {
			return errors.New("不能添加自己为好友哦")
		}
		if tfamID == familyID {
			return errors.New("你们属于同一个家庭，已经在「家庭」分组中啦！")
		}

		var already int
		_ = tx.QueryRow("SELECT 1 FROM parent_friendships WHERE parent_id = ? AND friend_parent_id = ?", func(r SQLiteRow) error {
			already = r.Int(0)
			return nil
		}, fromParentID, tid)
		if already == 1 {
			return errors.New("你们已经是好友啦！")
		}

		now := nowMs()
		var reverseReqID string
		_ = tx.QueryRow(
			"SELECT id FROM parent_friend_requests WHERE from_parent_id = ? AND to_parent_id = ? AND status = 'pending' LIMIT 1",
			func(r SQLiteRow) error {
				reverseReqID = r.String(0)
				return nil
			}, tid, fromParentID,
		)
		if reverseReqID != "" {
			if _, err := tx.Exec("UPDATE parent_friend_requests SET status = 'accepted', updated_at = ? WHERE id = ?", now, reverseReqID); err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO parent_friendships (parent_id, friend_parent_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", fromParentID, tid, now); err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO parent_friendships (parent_id, friend_parent_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", tid, fromParentID, now); err != nil {
				return err
			}
			statusOut = "accepted"
			noticeOut = "对方也发送过申请，你们已成为家长好友！"
			return nil
		}

		var existingReqID string
		_ = tx.QueryRow(
			"SELECT id FROM parent_friend_requests WHERE from_parent_id = ? AND to_parent_id = ? AND status = 'pending' LIMIT 1",
			func(r SQLiteRow) error {
				existingReqID = r.String(0)
				return nil
			}, fromParentID, tid,
		)
		if existingReqID != "" {
			if _, err := tx.Exec("UPDATE parent_friend_requests SET message = ?, updated_at = ? WHERE id = ?", cleanMsg, now, existingReqID); err != nil {
				return err
			}
			statusOut = "pending"
			noticeOut = fmt.Sprintf("已重新发送好友申请给 %s，等待对方同意", tname)
			return nil
		}

		if _, err := tx.Exec(
			"INSERT INTO parent_friend_requests (id, from_parent_id, to_parent_id, message, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', ?, ?)",
			GenID("pfreq_"), fromParentID, tid, cleanMsg, now, now,
		); err != nil {
			return err
		}
		statusOut = "pending"
		noticeOut = fmt.Sprintf("已向 %s 发送好友申请，等待对方同意", tname)
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return statusOut, noticeOut, nil
}

func (s *Store) RespondParentFriendRequest(parentID, reqID, action string) (string, error) {
	act := strings.ToLower(strings.TrimSpace(action))
	if act != "accept" && act != "reject" {
		return "", errors.New("无效的处理动作")
	}
	var notice string
	err := s.db.WithLock(func(tx *Tx) error {
		var fromID, toID, status string
		err := tx.QueryRow(
			"SELECT from_parent_id, to_parent_id, status FROM parent_friend_requests WHERE id = ?",
			func(r SQLiteRow) error {
				fromID = r.String(0)
				toID = r.String(1)
				status = r.String(2)
				return nil
			}, reqID,
		)
		if err != nil || toID != parentID {
			return errors.New("好友申请不存在")
		}
		if status != "pending" {
			return errors.New("该好友申请已处理")
		}
		now := nowMs()
		if act == "reject" {
			if _, err := tx.Exec("UPDATE parent_friend_requests SET status = 'rejected', updated_at = ? WHERE id = ?", now, reqID); err != nil {
				return err
			}
			notice = "已拒绝该家长好友申请"
			return nil
		}
		if _, err := tx.Exec("UPDATE parent_friend_requests SET status = 'accepted', updated_at = ? WHERE id = ?", now, reqID); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO parent_friendships (parent_id, friend_parent_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", parentID, fromID, now); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO parent_friendships (parent_id, friend_parent_id, group_name, created_at) VALUES (?, ?, '朋友', ?)", fromID, parentID, now); err != nil {
			return err
		}
		notice = "已同意好友申请，对方已加入「朋友」分组！"
		return nil
	})
	if err != nil {
		return "", err
	}
	return notice, nil
}

func (s *Store) RemoveParentFriend(parentID, peerID string) error {
	return s.db.WithLock(func(tx *Tx) error {
		affected, err := tx.Exec(
			"DELETE FROM parent_friendships WHERE (parent_id = ? AND friend_parent_id = ?) OR (parent_id = ? AND friend_parent_id = ?)",
			parentID, peerID, peerID, parentID,
		)
		if err != nil {
			return err
		}
		if affected == 0 {
			return errors.New("未找到该家长好友关系（家庭成员无法删除）")
		}
		return nil
	})
}

func (s *Store) GetChildSocialForParent(familyID, childID string) (*ChildSocialOverviewDTO, error) {
	var exists int
	_ = s.db.WithLock(func(tx *Tx) error {
		_ = tx.QueryRow("SELECT 1 FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			exists = r.Int(0)
			return nil
		}, childID, familyID)
		return nil
	})
	if exists != 1 {
		return nil, ErrNotFound
	}
	return s.GetChildSocialOverview(familyID, childID)
}

func (s *Store) ListChildChatForParent(familyID, childID, peerType, peerID string, afterMs int64) ([]ChatMessageDTO, error) {
	var list []ChatMessageDTO
	err := s.db.WithLock(func(tx *Tx) error {
		var exists int
		_ = tx.QueryRow("SELECT 1 FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			exists = r.Int(0)
			return nil
		}, childID, familyID)
		if exists != 1 {
			return ErrNotFound
		}
		allowed, err := s.canActorsChatLocked(tx, "child", childID, peerType, peerID)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("该孩子未参与该会话")
		}
		convKey := buildConvKey("child", childID, peerType, peerID)
		// 只读监护查看：不清除孩子端自己的未读红点
		list, err = s.fetchChatMessagesLocked(tx, "child", childID, convKey, afterMs)
		return err
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}
