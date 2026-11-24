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

type ChildSocialOverviewDTO struct {
	Self             ChildSocialSelfDTO `json:"self"`
	Groups           []string           `json:"groups"`
	Friends          []FriendItemDTO    `json:"friends"`
	IncomingRequests []FriendRequestDTO `json:"incoming_requests"`
	OutgoingRequests []FriendRequestDTO `json:"outgoing_requests"`
}

type ChatMessageDTO struct {
	ID           string `json:"id"`
	SenderType   string `json:"sender_type"`
	SenderID     string `json:"sender_id"`
	ReceiverType string `json:"receiver_type"`
	ReceiverID   string `json:"receiver_id"`
	IsSelf       bool   `json:"is_self"`
	MsgType      string `json:"msg_type"` // text | voice | image
	Text         string `json:"text"`
	MediaURL     string `json:"media_url,omitempty"`
	DurationSec  int    `json:"duration_sec"`
	IsRead       bool   `json:"is_read"`
	CreatedAt    int64  `json:"created_at"`
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
		err := tx.QueryRow("SELECT 1 FROM children WHERE friend_code = ?", func(r SQLiteRow) error {
			dummy = r.Int(0)
			return nil
		}, code)
		if err != nil || dummy == 0 {
			return code
		}
	}
	return fmt.Sprintf("GB%X", nowMs()%0xFFFFFF)
}

func buildConvKey(typeA, idA, typeB, idB string) string {
	a := typeA + ":" + idA
	b := typeB + ":" + idB
	if a <= b {
		return a + "|" + b
	}
	return b + "|" + a
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
			"SELECT msg_type, text_content, duration_sec, created_at FROM chat_messages WHERE conv_key = ? ORDER BY created_at DESC LIMIT 1",
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
		res, err := tx.Exec("UPDATE children SET friend_code = ? WHERE id = ? AND family_id = ?", code, childID, familyID)
		if err != nil {
			return err
		}
		if res.RowsAffected == 0 {
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
		err = errors.New("没有找到对应的孩子账号或分享码，请核对后重试")
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
		res, err := tx.Exec(
			"DELETE FROM friendships WHERE (child_id = ? AND friend_child_id = ?) OR (child_id = ? AND friend_child_id = ?)",
			childID, peerID, peerID, childID,
		)
		if err != nil {
			return err
		}
		if res.RowsAffected == 0 {
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

func (s *Store) ListChatMessages(viewerType, viewerID, peerType, peerID string, afterMs int64) ([]ChatMessageDTO, error) {
	list := make([]ChatMessageDTO, 0)
	convKey := buildConvKey(viewerType, viewerID, peerType, peerID)

	err := s.db.WithLock(func(tx *Tx) error {
		allowed, err := s.canActorsChatLocked(tx, viewerType, viewerID, peerType, peerID)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("你们还不是好友，无法查看聊天记录")
		}

		// 标记对方发给我的消息为已读
		_, _ = tx.Exec(
			"UPDATE chat_messages SET is_read = 1 WHERE conv_key = ? AND receiver_type = ? AND receiver_id = ? AND is_read = 0",
			convKey, viewerType, viewerID,
		)

		var q string
		var args []any
		if afterMs > 0 {
			q = `SELECT id, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at
				 FROM chat_messages
				 WHERE conv_key = ? AND created_at > ?
				 ORDER BY created_at ASC LIMIT 200`
			args = []any{convKey, afterMs}
		} else {
			q = `SELECT id, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at
				 FROM (
					SELECT id, sender_type, sender_id, receiver_type, receiver_id, msg_type, text_content, media_id, duration_sec, is_read, created_at
					FROM chat_messages
					WHERE conv_key = ?
					ORDER BY created_at DESC LIMIT 120
				 ) sub
				 ORDER BY created_at ASC`
			args = []any{convKey}
		}

		return tx.Query(q, func(r SQLiteRow) error {
			sType := r.String(1)
			sID := r.String(2)
			mID := r.String(7)
			mURL := ""
			if mID != "" {
				mURL = "/api/chat/media/" + mID
			}
			list = append(list, ChatMessageDTO{
				ID:           r.String(0),
				SenderType:   sType,
				SenderID:     sID,
				ReceiverType: r.String(3),
				ReceiverID:   r.String(4),
				IsSelf:       sType == viewerType && sID == viewerID,
				MsgType:      r.String(5),
				Text:         r.String(6),
				MediaURL:     mURL,
				DurationSec:  r.Int(8),
				IsRead:       r.Int(9) != 0,
				CreatedAt:    r.Int64(10),
			})
			return nil
		}, args...)
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

	err := s.db.WithLock(func(tx *Tx) error {
		allowed, err := s.canActorsChatLocked(tx, senderType, senderID, receiverType, receiverID)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("你们还不是好友，无法发送消息")
		}
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
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ChatMessageDTO{
		ID:           msgID,
		SenderType:   senderType,
		SenderID:     senderID,
		ReceiverType: receiverType,
		ReceiverID:   receiverID,
		IsSelf:       true,
		MsgType:      mType,
		Text:         cleanText,
		MediaURL:     mediaURL,
		DurationSec:  durationSec,
		IsRead:       false,
		CreatedAt:    now,
	}, nil
}

func (s *Store) GetChatMedia(viewerType, viewerID, mediaID string) (string, []byte, error) {
	var convKey, mimeType, dataB64 string
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow(
			"SELECT conv_key, mime_type, data_b64 FROM chat_media WHERE id = ?",
			func(r SQLiteRow) error {
				convKey = r.String(0)
				mimeType = r.String(1)
				dataB64 = r.String(2)
				return nil
			}, mediaID,
		)
	})
	if err != nil || dataB64 == "" {
		return "", nil, ErrNotFound
	}
	if !isParticipantOfConvKey(convKey, viewerType, viewerID) {
		return "", nil, errors.New("无权访问该聊天媒体")
	}
	raw, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return "", nil, ErrNotFound
	}
	return mimeType, raw, nil
}

func (s *Store) ListParentChatContacts(familyID, parentID string) ([]FriendItemDTO, error) {
	items := make([]FriendItemDTO, 0)
	err := s.db.WithLock(func(tx *Tx) error {
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
			return err
		}
		s.fillChatSummariesLocked(tx, "parent", parentID, items)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}
