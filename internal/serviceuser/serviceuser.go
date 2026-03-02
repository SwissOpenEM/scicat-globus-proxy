package serviceuser

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/paulscherrerinstitute/scicat-cli/v3/datasetUtils"
)

type ScicatServiceUser struct {
	scicatUrl   *string
	Username    *string
	Password    *string
	scicatToken *string
	expiry      *time.Time
	mutex       *sync.Mutex
}

func CreateServiceUser(scicatUrl string, username string, password string) (ScicatServiceUser, error) {
	var emptyString = ""
	var zeroTime = time.Time{}
	var mutex sync.Mutex
	serviceUser := ScicatServiceUser{
		scicatUrl:   &scicatUrl,
		Username:    &username,
		Password:    &password,
		scicatToken: &emptyString,
		expiry:      &zeroTime,
		mutex:       &mutex,
	}
	return serviceUser, serviceUser.refreshToken()
}

func (su *ScicatServiceUser) GetToken() (string, error) {
	su.mutex.Lock()
	defer su.mutex.Unlock()
	if time.Now().After(*su.expiry) {
		err := su.refreshToken()
		if err != nil {
			return "", err
		}
	}
	return *su.scicatToken, nil
}

func (su *ScicatServiceUser) refreshToken() error {
	user, _, err := datasetUtils.AuthenticateUser(http.DefaultClient, *su.scicatUrl+"api/v3", *su.Username, *su.Password, false)
	if err != nil {
		return err
	}

	token, ok := user["accessToken"]
	if !ok {
		return fmt.Errorf("token wasn't part of the user struct")
	}
	*su.scicatToken = token

	createdStr, ok := user["created"]
	if !ok {
		return fmt.Errorf("can't get the 'created' attribute from the userMap")
	}

	created, err := time.Parse(time.RFC3339, createdStr)
	if err != nil {
		return err
	}

	exipresInStr, ok := user["expiresIn"]
	if !ok {
		return fmt.Errorf("can't get the 'expiresIn' attribute from the userMap")
	}

	expiresIn, err := strconv.Atoi(exipresInStr)
	if err != nil {
		return err
	}

	*su.expiry = created.Add(time.Second * time.Duration(expiresIn))
	return nil
}
