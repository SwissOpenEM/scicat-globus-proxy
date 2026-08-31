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
		// Claims from the user's last OIDC login
		OidcClaims map[string]any `json:"oidcClaims"`
		ID_        string         `json:"_id"`
	} `json:"profile"`
	Provider    string    `json:"provider"`
	UserID      string    `json:"userId"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
	V           int       `json:"__v"`
	ScicatToken string
}
type ScicatDataset struct {
	Pid          string `json:"pid"`
	OwnerGroup   string `json:"ownerGroup"`
	SourceFolder string `json:"sourceFolder"`
}

type DataFile struct {
	Path string `json:"path"`
}

type ScicatOrigDatablock struct {
	Pid          string     `json:"datasetId"`
	DataFileList []DataFile `json:"dataFileList"`
}
