package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type UpstreamSessionInfo struct {
	AccessToken    string   `json:"access_token,omitempty"`
	APIToken       string   `json:"api_token,omitempty"`
	PlatformUserID int      `json:"platform_user_id,omitempty"`
	Username       string   `json:"username,omitempty"`
	Message        string   `json:"message,omitempty"`
	TokenNames     []string `json:"token_names,omitempty"`
}

var upstreamDigitRegexp = regexp.MustCompile(`\d{1,8}`)

func NormalizeUpstreamSite(site *model.UpstreamSite) error {
	if site == nil {
		return nil
	}
	site.BaseURL = normalizeUpstreamBaseURL(site.BaseURL)
	if site.BaseURL == "" {
		return errors.New("上游站点 base_url 不能为空")
	}

	if strings.TrimSpace(site.Platform) == "" || strings.TrimSpace(site.Preset) == "" {
		detection := DetectUpstreamChannelPreset(site.BaseURL, 0)
		if strings.TrimSpace(site.Platform) == "" {
			site.Platform = detection.Platform
		}
		if strings.TrimSpace(site.Preset) == "" && detection.Preset != nil {
			site.Preset = detection.Preset.ID
		}
	}

	if strings.TrimSpace(site.Name) == "" {
		if parsed, err := url.Parse(site.BaseURL); err == nil {
			site.Name = parsed.Host
		}
	}
	site.Name = strings.TrimSpace(site.Name)
	site.Platform = strings.TrimSpace(site.Platform)
	site.Preset = strings.TrimSpace(site.Preset)
	site.Proxy = strings.TrimSpace(site.Proxy)
	site.Status = strings.TrimSpace(site.Status)
	if site.Status == "" {
		site.Status = model.UpstreamSiteStatusActive
	}
	if site.Name == "" {
		return errors.New("上游站点名称不能为空")
	}
	if site.Platform == "" {
		return errors.New("未识别上游平台，请手动指定")
	}
	return nil
}

func ValidateUpstreamAccount(account *model.UpstreamAccount) error {
	if account == nil {
		return nil
	}
	account.Name = strings.TrimSpace(account.Name)
	account.Username = strings.TrimSpace(account.Username)
	account.AccessToken = strings.TrimSpace(account.AccessToken)
	account.APIToken = strings.TrimSpace(account.APIToken)
	account.Status = strings.TrimSpace(account.Status)
	if account.Status == "" {
		account.Status = model.UpstreamAccountStatusActive
	}
	if account.SiteID <= 0 {
		return errors.New("请选择上游站点")
	}
	if account.Name == "" {
		if account.Username != "" {
			account.Name = account.Username
		} else {
			account.Name = fmt.Sprintf("account-%d", common.GetTimestamp())
		}
	}
	if account.CheckinIntervalHours <= 0 {
		account.CheckinIntervalHours = 24
	}
	if account.AccessToken == "" && (account.Username == "" || account.Password == "") {
		return errors.New("请填写 access token，或填写用户名和密码后再刷新会话")
	}
	return nil
}

func RefreshUpstreamAccountSession(account *model.UpstreamAccount) (*UpstreamSessionInfo, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	site, err := loadUpstreamSiteForAccount(account)
	if err != nil {
		return nil, err
	}

	var session *UpstreamSessionInfo
	if account.Username != "" && account.Password != "" {
		session, err = loginUpstreamAccount(site, account.Username, account.Password)
		if err != nil {
			return nil, err
		}
	} else {
		session, err = verifyUpstreamAccountToken(site, account.AccessToken, account.PlatformUserID)
		if err != nil {
			return nil, err
		}
	}

	if session.AccessToken != "" {
		account.AccessToken = strings.TrimSpace(session.AccessToken)
	}
	if session.APIToken != "" {
		account.APIToken = strings.TrimSpace(session.APIToken)
	}
	if session.PlatformUserID > 0 {
		account.PlatformUserID = session.PlatformUserID
	}
	if session.Username != "" && account.Username == "" {
		account.Username = session.Username
	}
	account.Status = model.UpstreamAccountStatusActive
	account.UpdatedTime = common.GetTimestamp()
	if account.CheckinIntervalHours <= 0 {
		account.CheckinIntervalHours = 24
	}
	if account.Id == 0 {
		if err := account.Insert(); err != nil {
			return nil, err
		}
	} else {
		if err := account.Update(); err != nil {
			return nil, err
		}
	}
	account.Site = site
	return session, nil
}

func RunUpstreamAccountCheckin(account *model.UpstreamAccount, trigger string) (*model.UpstreamCheckinLog, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	site, err := loadUpstreamSiteForAccount(account)
	if err != nil {
		return nil, err
	}
	if !account.CheckinEnabled && trigger != "manual" {
		return persistUpstreamCheckinResult(account, &model.UpstreamCheckinLog{
			AccountID: account.Id,
			Status:    model.UpstreamCheckinStatusSkipped,
			Trigger:   trigger,
			Message:   "未启用自动签到",
		})
	}
	if strings.TrimSpace(account.AccessToken) == "" {
		if account.Username != "" && account.Password != "" {
			if _, err := RefreshUpstreamAccountSession(account); err != nil {
				return persistUpstreamCheckinResult(account, &model.UpstreamCheckinLog{
					AccountID: account.Id,
					Status:    model.UpstreamCheckinStatusFailed,
					Trigger:   trigger,
					Message:   err.Error(),
				})
			}
		} else {
			return persistUpstreamCheckinResult(account, &model.UpstreamCheckinLog{
				AccountID: account.Id,
				Status:    model.UpstreamCheckinStatusFailed,
				Trigger:   trigger,
				Message:   "账号会话不存在，请先刷新会话",
			})
		}
	}

	result, err := executeUpstreamCheckin(site, account, trigger)
	if err != nil {
		if shouldRetryUpstreamLogin(err.Error()) && account.Username != "" && account.Password != "" {
			if _, refreshErr := RefreshUpstreamAccountSession(account); refreshErr == nil {
				return executeUpstreamCheckin(site, account, trigger)
			}
		}
		return persistUpstreamCheckinResult(account, &model.UpstreamCheckinLog{
			AccountID: account.Id,
			Status:    model.UpstreamCheckinStatusFailed,
			Trigger:   trigger,
			Message:   err.Error(),
		})
	}
	return persistUpstreamCheckinResult(account, result)
}

func loginUpstreamAccount(site *model.UpstreamSite, username string, password string) (*UpstreamSessionInfo, error) {
	body, headers, _, err := doUpstreamJSONRequest(
		site,
		http.MethodPost,
		"/api/user/login",
		map[string]any{
			"username": username,
			"password": password,
		},
		http.Header{
			"X-Requested-With": []string{"XMLHttpRequest"},
		},
	)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{}
	if len(body) > 0 {
		_ = common.Unmarshal(body, &payload)
	}
	success, _ := payload["success"].(bool)
	if !success {
		return nil, errors.New(firstNonEmptyString(extractJSONMessage(payload), strings.TrimSpace(string(body)), "登录失败"))
	}

	accessToken := extractAccessTokenFromLoginPayload(payload)
	if accessToken == "" {
		accessToken = extractSessionCookie(headers)
	}
	if accessToken == "" {
		return nil, errors.New("登录成功，但未拿到可用会话")
	}

	session, err := verifyUpstreamAccountToken(site, accessToken, 0)
	if err != nil {
		return nil, err
	}
	if session.Username == "" {
		session.Username = username
	}
	if session.AccessToken == "" {
		session.AccessToken = accessToken
	}
	return session, nil
}

func verifyUpstreamAccountToken(site *model.UpstreamSite, accessToken string, preferredUserID int) (*UpstreamSessionInfo, error) {
	sitePlatform := normalizeUpstreamAccountPlatform(site.Platform)
	switch {
	case isNewAPIFamilyPlatform(sitePlatform):
		return verifyNewAPIFamilyToken(site, accessToken, preferredUserID)
	case isOneAPIFamilyPlatform(sitePlatform):
		return verifyOneAPIFamilyToken(site, accessToken)
	default:
		return nil, fmt.Errorf("当前平台暂未支持自动获取会话信息: %s", sitePlatform)
	}
}

func executeUpstreamCheckin(site *model.UpstreamSite, account *model.UpstreamAccount, trigger string) (*model.UpstreamCheckinLog, error) {
	sitePlatform := normalizeUpstreamAccountPlatform(site.Platform)
	checkinSupported := false
	if preset := findUpstreamPreset(sitePlatform); preset != nil {
		checkinSupported = preset.CheckinSupported
	}
	if !checkinSupported {
		return &model.UpstreamCheckinLog{
			AccountID: account.Id,
			Status:    model.UpstreamCheckinStatusSkipped,
			Trigger:   trigger,
			Message:   "该上游平台当前不支持签到",
		}, nil
	}

	if isNewAPIFamilyPlatform(sitePlatform) && account.PlatformUserID <= 0 {
		session, err := verifyNewAPIFamilyToken(site, account.AccessToken, 0)
		if err == nil && session.PlatformUserID > 0 {
			account.PlatformUserID = session.PlatformUserID
			if session.APIToken != "" {
				account.APIToken = session.APIToken
			}
			_ = account.Update()
		}
	}

	body, _, statusCode, err := doUpstreamJSONRequest(
		site,
		http.MethodPost,
		"/api/user/checkin",
		nil,
		buildUpstreamAccountHeaders(sitePlatform, account.AccessToken, account.PlatformUserID),
	)
	if err != nil {
		return nil, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code: %d %s", statusCode, strings.TrimSpace(string(body)))
	}

	payload := map[string]any{}
	if len(body) > 0 {
		if err := common.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("签到响应格式错误: %w", err)
		}
	}
	success, _ := payload["success"].(bool)
	message := extractJSONMessage(payload)
	reward := extractCheckinReward(payload)
	if success {
		return &model.UpstreamCheckinLog{
			AccountID: account.Id,
			Status:    model.UpstreamCheckinStatusSuccess,
			Trigger:   trigger,
			Message:   firstNonEmptyString(message, "签到成功"),
			Reward:    reward,
		}, nil
	}
	return &model.UpstreamCheckinLog{
		AccountID: account.Id,
		Status:    model.UpstreamCheckinStatusFailed,
		Trigger:   trigger,
		Message:   firstNonEmptyString(message, "签到失败"),
		Reward:    reward,
	}, nil
}

func persistUpstreamCheckinResult(account *model.UpstreamAccount, logEntry *model.UpstreamCheckinLog) (*model.UpstreamCheckinLog, error) {
	if logEntry == nil {
		return nil, errors.New("log entry is nil")
	}
	if logEntry.AccountID == 0 {
		logEntry.AccountID = account.Id
	}
	if err := logEntry.Insert(); err != nil {
		return nil, err
	}
	account.LastCheckinAt = logEntry.CreatedTime
	account.LastCheckinStatus = logEntry.Status
	account.LastCheckinMessage = logEntry.Message
	account.LastCheckinReward = logEntry.Reward
	switch logEntry.Status {
	case model.UpstreamCheckinStatusSuccess, model.UpstreamCheckinStatusSkipped:
		account.Status = model.UpstreamAccountStatusActive
	default:
		if shouldMarkAccountExpired(logEntry.Message) {
			account.Status = model.UpstreamAccountStatusExpired
		}
	}
	if account.Id > 0 {
		if err := account.Update(); err != nil {
			return nil, err
		}
	}
	return logEntry, nil
}

func loadUpstreamSiteForAccount(account *model.UpstreamAccount) (*model.UpstreamSite, error) {
	if account.Site != nil && account.Site.Id > 0 {
		return account.Site, nil
	}
	if account.SiteID <= 0 {
		return nil, errors.New("上游账号未绑定站点")
	}
	site, err := model.GetUpstreamSiteByID(account.SiteID)
	if err != nil {
		return nil, err
	}
	account.Site = site
	return site, nil
}

func normalizeUpstreamAccountPlatform(platform string) string {
	switch strings.TrimSpace(platform) {
	case UpstreamPlatformAnyRouter, UpstreamPlatformVeloera:
		return UpstreamPlatformNewAPI
	case UpstreamPlatformOneHub:
		return UpstreamPlatformOneAPI
	default:
		return strings.TrimSpace(platform)
	}
}

func isNewAPIFamilyPlatform(platform string) bool {
	switch platform {
	case UpstreamPlatformNewAPI:
		return true
	default:
		return false
	}
}

func isOneAPIFamilyPlatform(platform string) bool {
	switch platform {
	case UpstreamPlatformOneAPI:
		return true
	default:
		return false
	}
}

func verifyNewAPIFamilyToken(site *model.UpstreamSite, accessToken string, preferredUserID int) (*UpstreamSessionInfo, error) {
	credential := strings.TrimSpace(accessToken)
	if credential == "" {
		return nil, errors.New("access token 不能为空")
	}

	if session, ok := fetchNewAPIUserSelf(site, credential, 0); ok {
		session.AccessToken = credential
		session.APIToken = fetchPreferredNewAPIToken(site, credential, session.PlatformUserID)
		return session, nil
	}

	candidates := buildNewAPIUserIDCandidates(credential, preferredUserID)
	for _, candidate := range candidates {
		if session, ok := fetchNewAPIUserSelf(site, credential, candidate); ok {
			session.AccessToken = credential
			session.PlatformUserID = candidate
			session.APIToken = fetchPreferredNewAPIToken(site, credential, candidate)
			return session, nil
		}
	}

	return nil, errors.New("无法自动识别上游 user id，请确认会话是否有效")
}

func verifyOneAPIFamilyToken(site *model.UpstreamSite, accessToken string) (*UpstreamSessionInfo, error) {
	body, _, statusCode, err := doUpstreamJSONRequest(
		site,
		http.MethodGet,
		"/api/user/self",
		nil,
		buildUpstreamAccountHeaders(UpstreamPlatformOneAPI, accessToken, 0),
	)
	if err != nil {
		return nil, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code: %d %s", statusCode, strings.TrimSpace(string(body)))
	}
	payload := map[string]any{}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	success, _ := payload["success"].(bool)
	if !success {
		return nil, errors.New(firstNonEmptyString(extractJSONMessage(payload), "会话验证失败"))
	}
	data, _ := payload["data"].(map[string]any)
	session := &UpstreamSessionInfo{
		AccessToken: strings.TrimSpace(accessToken),
		Username:    extractUsername(data),
		APIToken:    fetchPreferredOneAPIToken(site, accessToken),
	}
	return session, nil
}

func fetchNewAPIUserSelf(site *model.UpstreamSite, accessToken string, userID int) (*UpstreamSessionInfo, bool) {
	body, _, statusCode, err := doUpstreamJSONRequest(
		site,
		http.MethodGet,
		"/api/user/self",
		nil,
		buildUpstreamAccountHeaders(UpstreamPlatformNewAPI, accessToken, userID),
	)
	if err != nil || statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return nil, false
	}
	payload := map[string]any{}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	success, _ := payload["success"].(bool)
	if !success {
		return nil, false
	}
	data, _ := payload["data"].(map[string]any)
	resolvedUserID := userID
	if resolvedUserID <= 0 {
		resolvedUserID = extractUserID(data)
	}
	if resolvedUserID <= 0 {
		return nil, false
	}
	return &UpstreamSessionInfo{
		PlatformUserID: resolvedUserID,
		Username:       extractUsername(data),
	}, true
}

func fetchPreferredNewAPIToken(site *model.UpstreamSite, accessToken string, userID int) string {
	token, _, _ := fetchUpstreamTokenList(site, buildUpstreamAccountHeaders(UpstreamPlatformNewAPI, accessToken, userID))
	return token
}

func fetchPreferredOneAPIToken(site *model.UpstreamSite, accessToken string) string {
	token, _, _ := fetchUpstreamTokenList(site, buildUpstreamAccountHeaders(UpstreamPlatformOneAPI, accessToken, 0))
	return token
}

func fetchUpstreamTokenList(site *model.UpstreamSite, headers http.Header) (string, []string, error) {
	body, _, statusCode, err := doUpstreamJSONRequest(site, http.MethodGet, "/api/token/?p=0&size=100", nil, headers)
	if err != nil {
		return "", nil, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return "", nil, fmt.Errorf("status code: %d", statusCode)
	}
	payload := map[string]any{}
	if err := common.Unmarshal(body, &payload); err != nil {
		return "", nil, err
	}
	items := extractUpstreamTokenItems(payload)
	tokenNames := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(fmt.Sprintf("%v", item["key"]))
		if key == "" || key == "<nil>" {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", item["name"]))
		if name == "" || name == "<nil>" {
			name = "default"
		}
		tokenNames = append(tokenNames, name)
		statusNumber, statusExists := toInt(item["status"])
		if !statusExists || statusNumber == 1 {
			return key, tokenNames, nil
		}
	}
	return "", tokenNames, nil
}

func extractUpstreamTokenItems(payload map[string]any) []map[string]any {
	candidates := []any{
		payload["data"],
		payload["items"],
		payload["list"],
	}
	if data, ok := payload["data"].(map[string]any); ok {
		candidates = append(candidates, data["items"], data["data"], data["list"])
	}
	for _, candidate := range candidates {
		if items, ok := toMapSlice(candidate); ok {
			return items
		}
	}
	return nil
}

func buildNewAPIUserIDCandidates(accessToken string, preferredUserID int) []int {
	candidates := make([]int, 0, 24)
	push := func(value int) {
		if value <= 0 {
			return
		}
		for _, existing := range candidates {
			if existing == value {
				return
			}
		}
		candidates = append(candidates, value)
	}
	push(preferredUserID)
	push(extractJWTUserID(accessToken))
	for _, match := range upstreamDigitRegexp.FindAllString(accessToken, -1) {
		if value, err := strconvAtoi(match); err == nil {
			push(value)
		}
	}
	for _, value := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 20, 50, 100} {
		push(value)
	}
	return candidates
}

func extractJWTUserID(accessToken string) int {
	raw := strings.TrimSpace(accessToken)
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return 0
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}
	payload := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return 0
	}
	if value := extractUserID(payload); value > 0 {
		return value
	}
	return 0
}

func buildUpstreamAccountHeaders(platform string, accessToken string, userID int) http.Header {
	headers := http.Header{}
	credential := strings.TrimSpace(accessToken)
	if credential != "" {
		if isCookieCredential(credential) {
			headers.Set("Cookie", normalizeCookieCredential(credential))
		} else if strings.HasPrefix(strings.ToLower(credential), "bearer ") {
			headers.Set("Authorization", credential)
		} else {
			headers.Set("Authorization", "Bearer "+credential)
		}
	}
	if userID > 0 {
		value := fmt.Sprintf("%d", userID)
		switch platform {
		case UpstreamPlatformNewAPI, UpstreamPlatformAnyRouter, UpstreamPlatformVeloera:
			headers.Set("New-Api-User", value)
			headers.Set("New-API-User", value)
			headers.Set("Veloera-User", value)
			headers.Set("voapi-user", value)
			headers.Set("User-id", value)
			headers.Set("Rix-Api-User", value)
			headers.Set("neo-api-user", value)
		default:
			headers.Set("New-Api-User", value)
			headers.Set("New-API-User", value)
			headers.Set("User-id", value)
		}
	}
	return headers
}

func doUpstreamJSONRequest(site *model.UpstreamSite, method string, path string, body any, headers http.Header) ([]byte, http.Header, int, error) {
	client, err := NewProxyHttpClient(site.Proxy)
	if err != nil {
		return nil, nil, 0, err
	}

	var reader io.Reader
	if body != nil {
		switch typed := body.(type) {
		case string:
			reader = strings.NewReader(typed)
		case []byte:
			reader = strings.NewReader(string(typed))
		default:
			payload, marshalErr := common.Marshal(body)
			if marshalErr != nil {
				return nil, nil, 0, marshalErr
			}
			reader = strings.NewReader(string(payload))
		}
	}

	req, err := http.NewRequest(method, strings.TrimSuffix(site.BaseURL, "/")+path, reader)
	if err != nil {
		return nil, nil, 0, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, resp.StatusCode, err
	}
	return respBody, resp.Header, resp.StatusCode, nil
}

func isCookieCredential(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "=") || strings.Contains(trimmed, ";")
}

func normalizeCookieCredential(value string) string {
	parts := strings.Split(strings.TrimSpace(value), ";")
	pairs := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "=") {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "path=") ||
			strings.HasPrefix(lower, "expires=") ||
			strings.HasPrefix(lower, "max-age=") ||
			strings.HasPrefix(lower, "domain=") ||
			strings.HasPrefix(lower, "httponly") ||
			strings.HasPrefix(lower, "secure") ||
			strings.HasPrefix(lower, "samesite=") {
			continue
		}
		pairs = append(pairs, trimmed)
	}
	return strings.Join(pairs, "; ")
}

func extractSessionCookie(headers http.Header) string {
	values := headers.Values("Set-Cookie")
	if len(values) == 0 {
		return ""
	}
	return normalizeCookieCredential(strings.Join(values, "; "))
}

func extractAccessTokenFromLoginPayload(payload map[string]any) string {
	data := payload["data"]
	switch typed := data.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		for _, key := range []string{"token", "access_token", "accessToken"} {
			if value, ok := typed[key]; ok {
				return strings.TrimSpace(fmt.Sprintf("%v", value))
			}
		}
	}
	return ""
}

func extractJSONMessage(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if value, ok := payload["message"]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
	if value, ok := payload["msg"]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
	return ""
}

func extractUserID(data map[string]any) int {
	if data == nil {
		return 0
	}
	for _, key := range []string{"id", "user_id", "userId"} {
		if value, ok := toInt(data[key]); ok {
			return value
		}
	}
	return 0
}

func extractUsername(data map[string]any) string {
	if data == nil {
		return ""
	}
	for _, key := range []string{"username", "display_name", "displayName", "email"} {
		value := strings.TrimSpace(fmt.Sprintf("%v", data[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func extractCheckinReward(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if data, ok := payload["data"].(map[string]any); ok {
		for _, key := range []string{"quota_awarded", "reward"} {
			if value, exists := data[key]; exists {
				return stringifyUpstreamReward(value)
			}
		}
	}
	return ""
}

func toMapSlice(value any) ([]map[string]any, bool) {
	rawItems, ok := value.([]any)
	if !ok {
		return nil, false
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, item)
	}
	return items, true
}

func toInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		number, err := typed.Int64()
		return int(number), err == nil
	case string:
		number, err := strconvAtoi(strings.TrimSpace(typed))
		return number, err == nil
	default:
		return 0, false
	}
}

func strconvAtoi(value string) (int, error) {
	var number int
	_, err := fmt.Sscanf(value, "%d", &number)
	return number, err
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shouldRetryUpstreamLogin(message string) bool {
	return shouldMarkAccountExpired(message)
}

func shouldMarkAccountExpired(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "401") ||
		strings.Contains(normalized, "403") ||
		strings.Contains(normalized, "expired") ||
		strings.Contains(normalized, "invalid token") ||
		strings.Contains(normalized, "unauthorized") ||
		strings.Contains(normalized, "forbidden") ||
		strings.Contains(normalized, "未登录") ||
		strings.Contains(normalized, "登录") ||
		strings.Contains(normalized, "过期")
}
