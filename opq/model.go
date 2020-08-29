package opq

import (
	"encoding/json"
	"fmt"
)

type AtMsg struct {
	Content string   `json:"Content,omitempty"`
	UserID  []uint64 `json:"UserID,omitempty"`
}

type PicMsg struct {
	AtMsg
	GroupPic []GroupPicInfo `json:"GroupPic,omitempty"`
	Tips     string         `json:"Tips,omitempty"`
}

type GroupPicInfo struct {
	FileID       int64  `json:"FileId,omitempty"`
	FileMd5      string `json:"FileMd5,omitempty"`
	FileSize     int    `json:"FileSize,omitempty"`
	ForwardBuf   string `json:"ForwordBuf,omitempty"`   //Note Forword shoule be a mistaken spelling, but we must keep it unchanged
	ForwardField int    `json:"ForwordField,omitempty"` //Note Forword shoule be a mistaken spelling, but we must keep it unchanged
	URL          string `json:"Url,omitempty"`
}

type BigFaceMsg struct {
	Content      string `json:"Content,omitempty"`
	ForwardBuf   string `json:"ForwordBuf,omitempty"`   //Note Forword shoule be a mistaken spelling, but we must keep it unchanged
	ForwardField int    `json:"ForwordField,omitempty"` //Note Forword shoule be a mistaken spelling, but we must keep it unchanged
	Tips         string `json:"Tips,omitempty"`
}

type GroupMessageEvent struct {
	CurrentPacket struct {
		Data      GroupMessageData `json:"Data,omitempty"`
		WebConnID string           `json:"WebConnId,omitempty"`
	}
	CurrentQQ uint64 `json:"CurrentQQ,omitempty"`
}

type GroupMessageData struct {
	Content       string `json:"Content,omitempty"`
	FromGroupID   uint64 `json:"FromGroupId,omitempty"`
	FromGroupName string `json:"FromGroupName,omitempty"`
	FromNickName  string `json:"FromNickName,omitempty"`
	FromUserID    uint64 `json:"FromUserId,omitempty"`
	MsgRandom     uint64 `json:"MsgRandom,omitempty"`
	MsgSeq        uint64 `json:"MsgSeq,omitempty"`
	MsgTime       uint64 `json:"MsgTime,omitempty"`
	MsgType       string `json:"MsgType,omitempty"`
}

type FriendMessageEvent struct {
	CurrentPacket struct {
		Data      FriendMessageData `json:"Data,omitempty"`
		WebConnID string            `json:"WebConnId,omitempty"`
	}
	CurrentQQ uint64 `json:"CurrentQQ,omitempty"`
}

type FriendMessageData struct {
	Content string `json:"Content,omitempty"`
	FromUin uint64 `json:"FromUin,omitempty"`
	ToUin   uint64 `json:"ToUin,omitempty"`
	MsgSeq  uint64 `json:"MsgSeq,omitempty"`
	MsgType string `json:"MsgType,omitempty"`
}

type UserInfoResponse struct {
	Code    int      `json:"code,omitempty"`
	Data    UserInfo `json:"data,omitempty"`
	Default int      `json:"default,omitempty"`
	Message string   `json:"message,omitempty"`
	Subcode int      `json:"subcode,omitempty"`
}

type UserInfo struct {
	AvatarURL     string `json:"avatarUrl,omitempty"`
	Bitmap        string `json:"bitmap,omitempty"`
	Commfrd       int    `json:"commfrd,omitempty"`
	Friendship    int    `json:"friendship,omitempty"`
	IntimacyScore int    `json:"intimacyScore,omitempty"`
	IsFriend      int    `json:"isFriend,omitempty"`
	Logolabel     string `json:"logolabel,omitempty"`
	Nickname      string `json:"nickname,omitempty"`
	Qzone         int    `json:"qzone,omitempty"`
	Realname      string `json:"realname,omitempty"`
	Smartname     string `json:"smartname,omitempty"`
	Uin           uint64 `json:"uin,omitempty"`
}

type GroupListResponse struct {
	Count     int         `json:"Count,omitempty"`
	NextToken string      `json:"NextToken,omitempty"`
	GroupList []GroupInfo `json:"TroopList,omitempty"`
}

type GroupInfo struct {
	GroupID    uint64 `json:"GroupId,omitempty"`
	GroupName  string `json:"GroupName,omitempty"`
	GroupOwner uint64 `json:"GroupOwner,omitempty"`
}

type MemberListResponse struct {
	GroupUin   uint64       `json:"GroupUin,omitempty"`
	LastUin    uint64       `json:"LastUin,omitempty"`
	Count      int          `json:"Count,omitempty"`
	MemberList []MemberInfo `json:"MemberList,omitempty"`
}

type MemberInfo struct {
	Age           int    `json:"Age,omitempty"`
	AutoRemark    string `json:"AutoRemark,omitempty"`
	CreditLevel   int    `json:"CreditLevel,omitempty"`
	Email         string `json:"Email,omitempty"`
	FaceID        int    `json:"FaceId,omitempty"`
	Gender        int    `json:"Gender,omitempty"`
	GroupAdmin    int    `json:"GroupAdmin,omitempty"`
	GroupCard     string `json:"GroupCard,omitempty"`
	JoinTime      uint64 `json:"JoinTime,omitempty"`
	LastSpeakTime uint64 `json:"LastSpeakTime,omitempty"`
	MemberLevel   int    `json:"MemberLevel,omitempty"`
	MemberUin     uint64 `json:"MemberUin,omitempty"`
	Memo          string `json:"Memo,omitempty"`
	NickName      string `json:"NickName,omitempty"`
	ShowName      string `json:"ShowName,omitempty"`
	SpecialTitle  string `json:"SpecialTitle,omitempty"`
	Status        int    `json:"Status,omitempty"`
}

type OPQErrorResponse struct {
	Ret int    `json:"Ret,omitempty"`
	Msg string `json:"Msg,omitempty"`
}

func (e OPQErrorResponse) Error() string {
	return fmt.Sprintf("[Code: %d] %s", e.Ret, e.Msg)
}

type EventMessagePacket struct {
	CurrentPacket struct {
		Data struct {
			EventData    json.RawMessage `json:"EventData,omitempty"`
			EventMessage EventMessage    `json:"EventMsg,omitempty"`
			EventName    string          `json:"EventName,omitempty"`
		} `json:"Data,omitempty"`
		WebConnID string `json:"WebConnId,omitempty"`
	}
	CurrentQQ uint64 `json:"CurrentQQ,omitempty"`
}

type EventMessage struct {
	FromUin uint64 `json:"FromUin,omitempty"`
	ToUin   uint64 `json:"ToUin,omitempty"`
	MsgType string `json:"MsgType,omitempty"`
	Content string `json:"Content,omitempty"`
}

const GroupExitEventName = "ON_EVENT_GROUP_EXIT"

type GroupExitEventData struct {
	UserID uint64 `json:"UserID,omitempty"`
}

const GroupJoinEventName = "ON_EVENT_GROUP_JOIN"

type GroupJoinEventData struct {
	InviteUin uint64 `json:"InviteUin,omitempty"`
	UserID    uint64 `json:"UserID,omitempty"`
	UserName  string `json:"UserName,omitempty"`
}

const FriendAddedEventName = "ON_EVENT_FRIEND_ADDED"

type FriendAddedEventData struct {
	UserID        uint64 `json:"UserID,omitempty"`
	FromType      int    `json:"FromType,omitempty"`
	Field9        int64  `json:"Field_9,omitempty"`
	Content       string `json:"Content,omitempty"`
	FromGroupID   uint64 `json:"FromGroupId,omitempty"`
	FromGroupName string `json:"FromGroupName,omitempty"`
	Action        int    `json:"Action,omitempty"`
}
