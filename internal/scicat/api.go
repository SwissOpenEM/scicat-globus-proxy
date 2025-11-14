// Scicat API types
// TODO: generate this from the SciCat backend repository
package scicat

import "time"

// Scicat User
type User struct {
	ID           string `json:"id"`
	AuthStrategy string `json:"authStrategy"`
	ExternalID   string `json:"externalId"`
	Profile      struct {
		DisplayName    string `json:"displayName"`
		Email          string `json:"email"`
		Username       string `json:"username"`
		ThumbnailPhoto string `json:"thumbnailPhoto"`
		ID             string `json:"id"`
		Emails         []struct {
			Value string `json:"value"`
		} `json:"emails"`
		AccessGroups []string `json:"accessGroups"`
		OidcClaims   struct {
			Exp               int      `json:"exp"`
			Iat               int      `json:"iat"`
			AuthTime          int      `json:"auth_time"`
			Jti               string   `json:"jti"`
			Iss               string   `json:"iss"`
			Aud               string   `json:"aud"`
			Sub               string   `json:"sub"`
			Typ               string   `json:"typ"`
			Azp               string   `json:"azp"`
			Sid               string   `json:"sid"`
			AtHash            string   `json:"at_hash"`
			Acr               string   `json:"acr"`
			EmailVerified     bool     `json:"email_verified"`
			AccessGroups      []string `json:"accessGroups"`
			Name              string   `json:"name"`
			PreferredUsername string   `json:"preferred_username"`
			GivenName         string   `json:"given_name"`
			FamilyName        string   `json:"family_name"`
			Email             string   `json:"email"`
		} `json:"oidcClaims"`
		ID_ string `json:"_id"`
	} `json:"profile"`
	Provider    string    `json:"provider"`
	UserID      string    `json:"userId"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
	V           int       `json:"__v"`
	ScicatToken string
}

type ScicatDataset struct {
	OwnerGroup   string `json:"ownerGroup"`
	SourceFolder string `json:"sourceFolder"`
}
