package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/UBotPlatform/UBot.Account.OPQAgent/opq"
	ubot "github.com/UBotPlatform/UBot.Common.Go"
	gosocketio "github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/patrickmn/go-cache"
)

var event *ubot.AccountEventEmitter
var botAddr string
var botQQStr string
var botQQ uint64
var userInfoCache = cache.New(10*time.Minute, 5*time.Minute)
var groupNameCache = cache.New(10*time.Minute, 5*time.Minute)
var memberNameCache = cache.New(10*time.Minute, 5*time.Minute)

func luaApiCaller(funcName string, data interface{}, response interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	url := "http://" + botAddr + "/v1/LuaApiCaller?funcname=" + funcName + "&timeout=10&qq=" + botQQStr
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(dataBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if response != nil {
		err = json.NewDecoder(resp.Body).Decode(response)
		if err != nil {
			return err
		}
	}
	return nil
}

func getUserInfo(uid string) (*opq.UserInfo, error) {
	vCached, cached := userInfoCache.Get(uid)
	if cached {
		return vCached.(*opq.UserInfo), nil
	}
	iUid, err := strconv.ParseUint(uid, 10, 64)
	if err != nil {
		return nil, err
	}
	data := make(map[string]interface{})
	data["UserID"] = iUid
	var response opq.UserInfoResponse
	err = luaApiCaller("GetUserInfo", data, &response)
	if err != nil {
		return nil, err
	}
	if response.Code != 0 {
		return nil, errors.New(response.Message)
	}
	userInfoCache.Set(uid, &response.Data, cache.DefaultExpiration)
	return &response.Data, nil
}
func getGroupNameByList(id string) (string, error) {
	data := make(map[string]interface{})
	data["NextToken"] = ""
	var response opq.GroupListResponse
	err := luaApiCaller("friendlist.GetTroopListReqV2", data, &response)
	if err != nil {
		return "", err
	}
	for _, group := range response.GroupList {
		groupNameCache.Set(fmt.Sprint(group.GroupID), group.GroupName, cache.DefaultExpiration)
	}
	vCached, cached := groupNameCache.Get(id)
	if cached {
		return vCached.(string), nil
	}
	return "", errors.New("cannot find the group")
}
func getGroupNameBySearch(id string) (string, error) {
	data := make(map[string]interface{})
	data["Content"] = id
	data["Page"] = 0
	var response []opq.GroupInfo
	err := luaApiCaller("SearchGroup", data, &response)
	if err != nil {
		return "", err
	}
	for _, group := range response {
		groupNameCache.Set(fmt.Sprint(group.GroupID), group.GroupName, cache.DefaultExpiration)
	}
	vCached, cached := groupNameCache.Get(id)
	if cached {
		return vCached.(string), nil
	}
	return "", errors.New("cannot find the group")
}
func getGroupName(id string) (string, error) {
	vCached, cached := groupNameCache.Get(id)
	if cached {
		return vCached.(string), nil
	}
	r, err := getGroupNameByList(id)
	if err == nil {
		return r, nil
	}
	r, err = getGroupNameBySearch(id)
	if err != nil {
		return "", err
	}
	return r, nil
}
func getUserName(id string) (string, error) {
	u, err := getUserInfo(id)
	if err != nil {
		return "", err
	}
	return u.Nickname, nil
}

type MsgPacket struct {
	Content      string
	PicUrl       string
	PicBase64    string
	ForwardField int
	ForwardBuf   string
}

func (p *MsgPacket) IsEmpty() bool {
	return p.Content == "" && p.PicUrl == "" && p.ForwardBuf == ""
}

func sendChatMessagePackets(msgType ubot.MsgType, source string, target string, packets []*MsgPacket) error {
	iSource, err := strconv.ParseUint(source, 10, 64)
	if err != nil {
		return err
	}
	iTarget, err := strconv.ParseUint(target, 10, 64)
	if err != nil {
		return err
	}
	for _, packet := range packets {
		data := make(map[string]interface{})
		switch {
		case packet.ForwardBuf != "":
			data["sendMsgType"] = "ForwordMsg"
			data["forwordBuf"] = packet.ForwardBuf
			data["forwordField"] = packet.ForwardField
		case packet.PicUrl != "":
			data["sendMsgType"] = "PicMsg"
			data["picUrl"] = packet.PicUrl
			data["picBase64Buf"] = ""
			data["fileMd5"] = ""
			data["flashPic"] = 0
		case packet.PicBase64 != "":
			data["sendMsgType"] = "PicMsg"
			data["picUrl"] = ""
			data["picBase64Buf"] = packet.PicBase64
			data["fileMd5"] = ""
			data["flashPic"] = 0
		default:
			data["sendMsgType"] = "TextMsg"
		}
		data["content"] = packet.Content // must be set, even for ForwordMsg
		data["atUser"] = 0
		switch msgType {
		case ubot.GroupMsg:
			data["toUser"] = iSource
			data["sendToType"] = 2
			data["groupid"] = 0
		case ubot.PrivateMsg:
			data["toUser"] = iTarget
			data["sendToType"] = 1
			data["groupid"] = iSource
		}
		var response opq.OPQErrorResponse
		err := luaApiCaller("SendMsg", data, &response)
		if err != nil {
			return err
		}
		if response.Ret != 0 {
			return response
		}
	}
	return nil
}

func sendChatMessage(msgType ubot.MsgType, source string, target string, message string) error {
	entities := ubot.ParseMsg(message)
	packets := make([]*MsgPacket, 0, 2)
	packet := &MsgPacket{}
	imagePacket := func(setter func()) {
		if !packet.IsEmpty() {
			if packet.PicUrl != "" || packet.PicBase64 != "" {
				packets = append(packets, packet)
				packet = &MsgPacket{}
				setter()
			} else if packet.Content != "" { //由于第一个判断不成立，此时消息处于无图状态
				packet.Content = "[PICFLAG]" + packet.Content
				setter()
				packets = append(packets, packet)
				packet = &MsgPacket{}
			} else {
				packets = append(packets, packet)
				packet = &MsgPacket{}
				setter()
			}
		} else {
			setter()
		}
	}
	for _, entity := range entities {
		switch entity.Type {
		case "text":
			packet.Content += entity.Data
		case "face":
			packet.Content += fmt.Sprintf("[表情%s]", entity.Data)
		case "at":
			if entity.Data == "all" {
				packet.Content += "[ATALL()]"
			} else {
				packet.Content += fmt.Sprintf("[ATUSER(%s)]", entity.Data)
			}
		case "image_online":
			imagePacket(func() {
				packet.PicUrl = entity.Data
			})
		case "image_base64":
			imagePacket(func() {
				packet.PicBase64 = entity.Data
			})
		case "big_face":
			if !packet.IsEmpty() {
				packets = append(packets, packet)
				packet = &MsgPacket{}
			}
			pComma := strings.IndexByte(entity.Data, ',')
			if pComma == -1 {
				return errors.New("invaild big_face entity")
			}
			forwardField, err := strconv.Atoi(entity.Data[:pComma])
			if err != nil {
				return errors.New("invaild big_face entity")
			}
			forwardBuf := entity.Data[pComma+1:]
			packets = append(packets, &MsgPacket{ForwardField: forwardField, ForwardBuf: forwardBuf})
		}
	}
	if !packet.IsEmpty() {
		packets = append(packets, packet)
		packet = nil
	}
	return sendChatMessagePackets(msgType, source, target, packets)
}

func removeMember(source string, target string) error {
	iSource, err := strconv.ParseUint(source, 10, 64)
	if err != nil {
		return err
	}
	iTarget, err := strconv.ParseUint(target, 10, 64)
	if err != nil {
		return err
	}
	data := make(map[string]interface{})
	data["ActionType"] = 3
	data["GroupID"] = iSource
	data["ActionUserID"] = iTarget
	data["Content"] = ""
	var response opq.OPQErrorResponse
	err = luaApiCaller("GroupMgr", data, &response)
	if err != nil {
		return err
	}
	if response.Ret != 0 {
		return response
	}
	return nil
}
func shutupMember(source string, target string, duration int) error {
	iSource, err := strconv.ParseUint(source, 10, 64)
	if err != nil {
		return err
	}
	iTarget, err := strconv.ParseUint(target, 10, 64)
	if err != nil {
		return err
	}
	data := make(map[string]interface{})
	data["GroupID"] = iSource
	data["ShutUpUserID"] = iTarget
	data["ShutTime"] = duration
	var response opq.OPQErrorResponse
	err = luaApiCaller("OidbSvc.0x570_8", data, &response)
	if err != nil {
		return err
	}
	if response.Ret != 0 {
		return response
	}
	return nil
}
func shutupAllMember(source string, shutupSwitch bool) error {
	iSource, err := strconv.ParseUint(source, 10, 64)
	if err != nil {
		return err
	}
	data := make(map[string]interface{})
	data["GroupID"] = iSource
	if shutupSwitch {
		data["Switch"] = 1
	} else {
		data["Switch"] = 0
	}
	var response opq.OPQErrorResponse
	err = luaApiCaller("OidbSvc.0x89a_0", data, &response)
	if err != nil {
		return err
	}
	if response.Ret != 0 {
		return response
	}
	return nil
}

func getMemberName(source string, target string) (string, error) {
	vCached, cached := memberNameCache.Get(fmt.Sprintf("%s.%s", source, target))
	if cached {
		return vCached.(string), nil
	}
	return getUserName(target) // fallback
}

func getUserAvatar(id string) (string, error) {
	u, err := getUserInfo(id)
	if err != nil {
		return "", err
	}
	return u.AvatarURL, nil
}
func getSelfID() (string, error) {
	return botQQStr, nil
}

func getPlatformID() (string, error) {
	return "QQ", nil
}
func getGroupList() ([]string, error) {
	var r []string
	data := make(map[string]interface{})
	data["NextToken"] = ""
	var response opq.GroupListResponse
	err := luaApiCaller("friendlist.GetTroopListReqV2", data, &response)
	if err != nil {
		return nil, err
	}
	for _, group := range response.GroupList {
		groupNameCache.Set(fmt.Sprint(group.GroupID), group.GroupName, cache.DefaultExpiration)
		r = append(r, fmt.Sprint(group.GroupID))
	}
	return r, nil
}
func getMemberList(id string) ([]string, error) {
	var r []string
	var err error
	data := make(map[string]interface{})
	data["GroupUin"], err = strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, err
	}
	data["LastUin"] = 0
	var response opq.MemberListResponse
	err = luaApiCaller("friendlist.GetTroopMemberListReq", data, &response)
	if err != nil {
		return nil, err
	}
	for _, member := range response.MemberList {
		groupCard := member.GroupCard
		nickName := member.NickName
		if groupCard == "" {
			groupCard = nickName
		}
		memberNameCache.Set(fmt.Sprintf("%d.%d", response.GroupUin, member.MemberUin), groupCard, cache.DefaultExpiration)
		r = append(r, fmt.Sprint(member.MemberUin))
	}
	return r, nil
}

var atMsgDefaultMatcher = regexp.MustCompile(`@.*? `)

// 既然OPQ没有良好的转义机制，我们直接在转义后替换
var escapedFaceMsgMatcher = regexp.MustCompile(`\\\[表情(\d+)\\\]`)

func convertAtMessage(parsed *opq.AtMsg) string {
	msg := new(ubot.MsgBuilder).WriteString(parsed.Content).String()
	msg = escapedFaceMsgMatcher.ReplaceAllString(msg, "[face:$1]")
	i := 0
	userToReplace := make([]string, 0, 5)
	for _, user := range parsed.UserID {
		if user == 0 {
			msg = strings.ReplaceAll(msg, "@全体成员", "[at:all]")
			continue
		}
		userStr := fmt.Sprint(user)
		nick, err := getUserName(userStr)
		if err != nil {
			userToReplace = append(userToReplace, userStr)
			continue
		}
		desc := "@" + new(ubot.MsgBuilder).WriteString(nick).String()
		start := strings.Index(msg, desc)
		if start == -1 {
			userToReplace = append(userToReplace, userStr)
			continue
		}
		msg = msg[0:start] + "[at:" + userStr + "]" + msg[start+len(desc):]
	}

	if len(userToReplace) > 0 {
		// very stupid but can handle most cases
		msg = atMsgDefaultMatcher.ReplaceAllStringFunc(msg, func(match string) string {
			if i >= len(userToReplace) {
				return match
			} else {
				i++
				return "[at:" + userToReplace[i-1] + "] "
			}
		})
	}
	return msg
}

func convertMessage(opqMsgType string, opqMsg string) (string, error) {
	var builder ubot.MsgBuilder
	var err error
	var msg string
	switch opqMsgType {
	case "TextMsg":
		msg = new(ubot.MsgBuilder).WriteString(opqMsg).String()
		msg = escapedFaceMsgMatcher.ReplaceAllString(msg, "[face:$1]")
	case "AtMsg":
		var parsed opq.AtMsg
		err = json.Unmarshal([]byte(opqMsg), &parsed)
		if err != nil {
			return "", err
		}
		msg = convertAtMessage(&parsed)
	case "PicMsg":
		var parsed opq.PicMsg
		err = json.Unmarshal([]byte(opqMsg), &parsed)
		if err != nil {
			return "", err
		}
		msg = convertAtMessage(&parsed.AtMsg)
		for _, pic := range parsed.GroupPic {
			msg = fmt.Sprintf("%s[image_online:%s]", msg, pic.URL)
		}
	case "BigFaceMsg":
		var parsed opq.BigFaceMsg
		err = json.Unmarshal([]byte(opqMsg), &parsed)
		if err != nil {
			return "", err
		}
		msg = fmt.Sprintf("[big_face:%d,%s]", parsed.ForwardField, parsed.ForwardBuf)
	default:
		fmt.Fprintf(os.Stderr, "Unknown message type: %s, content: %s\n", opqMsgType, opqMsg)
		builder.WriteString(opqMsg)
		return "", fmt.Errorf("unknown message type: %s", opqMsgType)
	}
	return msg, nil
}

func main() {
	var err error
	botAddr = os.Args[3]
	botQQStr = os.Args[4]
	botQQ, err = strconv.ParseUint(botQQStr, 10, 64)
	ubot.AssertNoError(err)
	var botConn *gosocketio.Client
	opqConnected := false
	opcAcked := false
	for retryCount := 0; retryCount < 5; retryCount++ {
		botConn, err = gosocketio.Dial(
			"ws://"+botAddr+"/socket.io/?EIO=3&transport=websocket",
			transport.GetDefaultWebsocketTransport())
		if err == nil {
			opqConnected = true
			break
		}
		fmt.Println("Failed to connect to OPQ Server, it will try again in 5 seconds.")
		time.Sleep(5 * time.Second)
	}
	if !opqConnected {
		panic(errors.New("Failed to connect to OPQ Server after 5 attempts."))
	}
	for retryCount := 0; retryCount < 5; retryCount++ {
		ackResult, err := botConn.Ack("GetWebConn", botQQStr, 5*time.Second)
		if ackResult == "\"OK\"" && err == nil {
			opcAcked = true
			break
		}
		fmt.Println("Failed to ack, it will try again in 5 seconds.")
		time.Sleep(5 * time.Second)
	}
	if !opcAcked {
		panic(errors.New("Failed to ack after 5 attempts."))
	}
	_ = botConn.On("OnGroupMsgs", func(h *gosocketio.Channel, e opq.GroupMessageEvent) {
		data := &e.CurrentPacket.Data
		groupIDStr := fmt.Sprint(data.FromGroupID)
		groupNameCache.Set(groupIDStr, &data.FromGroupName, cache.DefaultExpiration)
		memberNameCache.Set(fmt.Sprintf("%d.%d", data.FromGroupID, data.FromUserID), data.FromNickName, cache.DefaultExpiration)
		if data.FromUserID == botQQ {
			return
		}
		msgId := fmt.Sprintf("group%d.%d.%d.%d", data.FromGroupID, data.MsgTime, data.MsgSeq, data.MsgRandom)
		msg, err := convertMessage(data.MsgType, data.Content)
		if err != nil {
			return
		}
		_ = event.OnReceiveChatMessage(ubot.GroupMsg,
			groupIDStr,
			fmt.Sprint(data.FromUserID),
			msg,
			ubot.MsgInfo{ID: msgId})
	})
	_ = botConn.On("OnFriendMsgs", func(h *gosocketio.Channel, e opq.FriendMessageEvent) {
		var err error
		data := &e.CurrentPacket.Data
		if data.FromUin == botQQ {
			return
		}
		msgId := fmt.Sprintf("friend%d.%d", data.FromUin, data.MsgSeq)
		msg, err := convertMessage(data.MsgType, data.Content)
		if err != nil {
			return
		}
		_ = event.OnReceiveChatMessage(ubot.PrivateMsg,
			"",
			fmt.Sprint(data.FromUin),
			msg,
			ubot.MsgInfo{ID: msgId})
	})
	_ = botConn.On("OnEvents", func(h *gosocketio.Channel, e opq.EventMessagePacket) {
		var err error
		data := &e.CurrentPacket.Data
		switch data.EventName {
		case opq.GroupJoinEventName:
			var eventData opq.GroupJoinEventData
			err = json.Unmarshal(data.EventData, &eventData)
			if err != nil {
				return
			}
			if eventData.InviteUin == 0 {
				_ = event.OnMemberJoined(
					fmt.Sprint(data.EventMessage.FromUin),
					fmt.Sprint(eventData.UserID),
					"")
			} else {
				_ = event.OnMemberJoined(
					fmt.Sprint(data.EventMessage.FromUin),
					fmt.Sprint(eventData.UserID),
					fmt.Sprint(eventData.InviteUin))
			}
		case opq.GroupExitEventName:
			var eventData opq.GroupExitEventData
			err = json.Unmarshal(data.EventData, &eventData)
			if err != nil {
				return
			}
			_ = event.OnMemberLeft(
				fmt.Sprint(data.EventMessage.FromUin),
				fmt.Sprint(eventData.UserID))
		case opq.FriendAddedEventName:
			var eventData opq.FriendAddedEventData
			err = json.Unmarshal(data.EventData, &eventData)
			if err != nil {
				return
			}
			var result ubot.EventResultType
			result, _, err = event.ProcessFriendRequest(
				fmt.Sprint(eventData.UserID),
				fmt.Sprint(eventData.Content))
			if err != nil {
				return
			}
			switch result {
			case ubot.AcceptRequest:
				eventData.Action = 2
			case ubot.RejectRequest:
				eventData.Action = 3
			default:
				return
			}
			_ = luaApiCaller("DealFriend", eventData, nil)
		}
	})
	err = ubot.HostAccount("QQ"+botQQStr, func(e *ubot.AccountEventEmitter) *ubot.Account {
		event = e
		return &ubot.Account{
			GetGroupName:    getGroupName,
			GetUserName:     getUserName,
			SendChatMessage: sendChatMessage,
			RemoveMember:    removeMember,
			ShutupMember:    shutupMember,
			ShutupAllMember: shutupAllMember,
			GetMemberName:   getMemberName,
			GetUserAvatar:   getUserAvatar,
			GetSelfID:       getSelfID,
			GetPlatformID:   getPlatformID,
			GetGroupList:    getGroupList,
			GetMemberList:   getMemberList,
		}
	})
	ubot.AssertNoError(err)
}
