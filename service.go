package servicenowclient 

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/panho66/log4go"
	"io"
	"net/http"
	"net/url"
)

const CipherKey = "0123456789012345"

type Client struct {
	Username, Password, Instance, Key, Proxy string
}

/*
   Err represents a possible error message that came back from the server
   {"error":{"message":"Invalid table ss","detail":null},"status":"failure"}
*/
type Err struct {
	Status string    `json:status`
	Reason ErrDetail `json:"error"`
}

/*
   ErrDetail represents error message and detail
   {"error":{"message":"Invalid table ss","detail":null},"status":"failure"}
*/
type ErrDetail struct {
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

/*

 */
func (e Err) Error() string {
	return fmt.Sprintf("status=%s message=%s detail=%s", e.Status, e.Reason.Message, e.Reason.Detail)
}

func (c *Client) GetUserName() string {
	key := CipherKey
	if c.Key != "" {
		key = c.Key
	}
	CIPHER_KEY := []byte(key)
	decrypted, err := decrypt(CIPHER_KEY, c.Username)
	if err != nil {
		log.Error("msg='Failed to decrypt UserName'")
	}
	//log.Info(fmt.Sprintf("Username=%s encrypted=%s decrypt=%s", c.Username,encrypted,decrypted))
	return decrypted
}
func (c *Client) GetPassword() string {
	key := CipherKey
	if c.Key != "" {
		key = c.Key
	}
	CIPHER_KEY := []byte(key)
	decrypted, err := decrypt(CIPHER_KEY, c.Password)
	if err != nil {
		log.Error("msg='Failed to decrypt Password'")
	}
	//log.Info(fmt.Sprintf("Password=%s encrypted=%s decrypt=%s", c.Password,encrypted,decrypted))
	return decrypted
}

func decrypt(key []byte, securemess string) (decodedmess string, err error) {
	cipherText, err := base64.URLEncoding.DecodeString(securemess)
	if err != nil {
		return
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	if len(cipherText) < aes.BlockSize {
		err = errors.New("Ciphertext block size is too short!")
		return
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(cipherText, cipherText)

	decodedmess = string(cipherText)
	return
}
func encrypt(key []byte, message string) (encmess string, err error) {
	plainText := []byte(message)

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plainText)

	//returns to base64 encoded string
	encmess = base64.URLEncoding.EncodeToString(cipherText)
	return
}

/*
 PerformFor creates and executes an authenticated HTTP request to ServiceNow, and unmarhals the JSON
 into the passed output interface pointer, returning an error.
 input:
    table string serviceNow table
    opts url.Values name and value pair parameters to retrieve change record
    body interface{}
    out interface{}
 return:
    err if any
*/
func (c *Client) PerformFor(table string, opts url.Values, body interface{}, out interface{}) error {

	// https://iag.service-now.com/api/now/v2/table/change_request
	u := fmt.Sprintf("https://%s/%s?sysparm_query=%s", c.Instance, table, opts.Encode())
	log.Debug("url=" + u)

	var client *http.Client
	if c.Proxy != "" {
		proxyURL, err := url.Parse(c.Proxy)
		if err == nil {
			transport := &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
			client = &http.Client{
				Transport: transport,
			}
			
		} else {
			log.Debug("parse proxyURL failed. proxyURL=",c.Proxy,err)
			client = &http.Client{}
		}
	} else {
		client = &http.Client{}
	}
	req, err := http.NewRequest("GET", u, nil)
	req.SetBasicAuth(c.GetUserName(), c.GetPassword())
	resp, err := client.Do(req)
	if err != nil {
		log.Critical("HttpGet failed. Error=", err)
		return err
	}
	defer resp.Body.Close()
	// Store JSON so we can do a preliminary error check
	var echeck Err
	buf := &bytes.Buffer{}
	buf.Reset()
	err = json.NewDecoder(io.TeeReader(resp.Body, buf)).Decode(&echeck)
	log.Debug("returnjson=" + string(buf.String()))
	if err == nil && echeck.Status != "" {
		log.Error("unmarhals json error data failed. Error=", echeck)
		return echeck
	}
	err = json.NewDecoder(buf).Decode(out)
	if err != nil {
		log.Error("unmarhals json data failed. ", err)
	}
	return err
}

/*
   Get serviceNow record for a talbe, options and unmarhals JSON into out parameter
   input:
      table string serviceNow table
      opts  url.Values name and value pair parameters to retrieve change record
      out interface{}
   return:
      err
*/
func (c Client) GetRecordsFor(table string, opts url.Values, out interface{}) error {
	return c.PerformFor(table, opts, nil, out)
}
