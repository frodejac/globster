package google

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
)

const (
	stateCookieName = "gstate"
)

func (a *Auth) getUserInfo(client *http.Client) (*UserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo UserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, err
	}
	return &userInfo, nil
}

func (a *Auth) verifyDomain(userinfo *UserInfo) bool {
	if len(a.allowedDomains) == 0 {
		return true
	}
	emailParts := strings.Split(userinfo.Email, "@")
	if len(emailParts) != 2 || !slices.Contains(a.allowedDomains, emailParts[1]) {
		return false
	}
	return true
}

func (a *Auth) verifyGroupMembership(userinfo *UserInfo) bool {
	if len(a.allowedGroups) == 0 {
		return true
	}
	for _, group := range a.allowedGroups {
		if _, err := a.adminService.Members.Get(group, userinfo.Email).Do(); err == nil {
			return true
		}
	}
	return true
}
