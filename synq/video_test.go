package synq

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	VIDEO_ID          = "45d4063d00454c9fb21e5186a09c3115"
	VIDEO_ID2         = "55d4062f99454c9fb21e5186a09c2115"
	API_KEY           = "aba179c14ab349e0bb0d12b7eca5fa24"
	API_KEY2          = "cba179c14ab349e0bb0d12b7eca5fa25"
	UPLOAD_KEY        = "projects/0a/bf/0abfe1b849154082993f2fce77a16fd9/uploads/videos/55/d4/55d4062f99454c9fb21e5186a09c2115.mp4"
	INVALID_UUID      = `{"url": "http://docs.synq.fm/api/v1/errors/invalid_uuid","name": "invalid_uuid","message": "Invalid uuid. Example: '1c0e3ea4529011e6991554a050defa20'."}`
	VIDEO_NOT_FOUND   = `{"url": "http://docs.synq.fm/api/v1/errors/not_found_video","name": "not_found_video","message": "Video not found."}`
	API_KEY_NOT_FOUND = `{"url": "http://docs.synq.fm/api/v1/errors/not_found_api_key","name": "not_found_api_key","message": "API key not found."}`
	HTTP_NOT_FOUND    = `{"url": "http://docs.synq.fm/api/v1/errors/http_not_found","name": "http_not_found","message": "Not found."}`
)

func validKey(key string) string {
	if len(key) != 32 {
		return INVALID_UUID
	} else if key != API_KEY {
		return API_KEY_NOT_FOUND
	}
	return ""
}

func validVideo(id string) string {
	if len(id) != 32 {
		return INVALID_UUID
	} else if id != VIDEO_ID {
		return VIDEO_NOT_FOUND
	}
	return ""
}

func SynqStub() *httptest.Server {
	var resp []byte
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("here in synq response", r.RequestURI)
		testReqs = append(testReqs, r)
		if r.Method == "POST" {
			bytes, _ := ioutil.ReadAll(r.Body)
			//Parse response body
			v, _ := url.ParseQuery(string(bytes))
			testValues = append(testValues, v)
			key := v.Get("api_key")
			ke := validKey(key)
			if ke != "" {
				w.WriteHeader(http.StatusBadRequest)
				resp = []byte(ke)
			} else {
				switch r.RequestURI {
				case "/v1/video/details":
					video_id := v.Get("video_id")
					ke = validVideo(video_id)
					if ke != "" {
						w.WriteHeader(http.StatusBadRequest)
						resp = []byte(ke)
					} else {
						resp, _ = ioutil.ReadFile("../sample/video.json")
					}
				case "/v1/video/create":
					resp, _ = ioutil.ReadFile("../sample/new_video.json")
				case "/v1/video/upload":
					resp, _ = ioutil.ReadFile("../sample/upload.json")
				case "/v1/video/uploader":
					resp, _ = ioutil.ReadFile("../sample/uploader.json")
				case "/v1/video/update":
					resp, _ = ioutil.ReadFile("../sample/update.json")
				default:
					w.WriteHeader(http.StatusBadRequest)
					resp = []byte(HTTP_NOT_FOUND)
				}
			}
		}
		w.Write(resp)
	}))
}

func TestDisplay(t *testing.T) {
	assert := assert.New(t)
	p := Player{EmbedUrl: "url", ThumbnailUrl: "url2"}
	v := Video{}
	assert.Equal("Empty Video\n", v.Display())
	v.State = "created"
	assert.Equal("Empty Video\n", v.Display())
	v.Id = "abc123"
	assert.Equal("Video abc123\n\tState : created\n", v.Display())
	v.State = "uploading"
	assert.Equal("Video abc123\n\tState : uploading\n", v.Display())
	v.State = "uploaded"
	v.Player = p
	assert.Equal("Video abc123\n\tState : uploaded\n\tEmbed URL : url\n\tThumbnail : url2\n", v.Display())
}

func TestGetUploadInfo(t *testing.T) {
	assert := assert.New(t)
	api := setupTestApi("fake", false)
	video := Video{Id: VIDEO_ID2, Api: &api}
	err := video.GetUploadInfo()
	assert.NotNil(err)
	assert.Equal("Invalid uuid. Example: '1c0e3ea4529011e6991554a050defa20'.", err.Error())
	api.Key = API_KEY
	err = video.GetUploadInfo()
	assert.Nil(err)
	assert.NotEmpty(video.UploadInfo)
	assert.Len(video.UploadInfo, 7)
	assert.Equal(UPLOAD_KEY, video.UploadInfo["key"])
	assert.Equal("public-read", video.UploadInfo["acl"])
	assert.Equal("https://synqfm.s3.amazonaws.com", video.UploadInfo.url())
	assert.Equal("video/mp4", video.UploadInfo["Content-Type"])
	assert.Equal(UPLOAD_KEY, video.UploadInfo.dstFileName())
}

func TestCreateUploadReqErr(t *testing.T) {
	assert := assert.New(t)
	uploadInfo := make(Upload)
	valid_file := "video.go"
	_, err := uploadInfo.createUploadReq(valid_file)
	assert.NotNil(err)
	assert.Equal("no valid upload data", err.Error())
	uploadInfo["action"] = ":://noprotocol.com"
	uploadInfo["key"] = "fake"
	_, err = uploadInfo.createUploadReq(valid_file)
	assert.NotNil(err)
	assert.Equal("parse :://noprotocol.com: missing protocol scheme", err.Error())
	uploadInfo["action"] = "http://valid.com"
	_, err = uploadInfo.createUploadReq("myfile.mp4")
	assert.NotNil(err)
	assert.Equal("file 'myfile.mp4' does not exist", err.Error())
}

func TestCreateUploadReq(t *testing.T) {
	assert := assert.New(t)
	valid_file := "video.go"
	var uploadInfo Upload
	data, err := ioutil.ReadFile("../sample/upload.json")
	assert.Nil(err)
	err = json.Unmarshal(data, &uploadInfo)
	assert.Nil(err)
	req, err := uploadInfo.createUploadReq(valid_file)
	assert.Nil(err)
	assert.NotEmpty(req.Header)
	assert.Contains(req.Header.Get("Content-Type"), "multipart/form-data")
	f, h, err := req.FormFile("file")
	assert.Nil(err)
	assert.Equal(valid_file, h.Filename)
	assert.Nil(err)
	src, _ := ioutil.ReadFile(valid_file)
	b := make([]byte, len(src))
	f.Read(b)
	assert.Equal(src, b)
	assert.Len(req.PostForm, 6)
	for key, value := range uploadInfo {
		if key == "action" {
			assert.Equal("", req.PostFormValue(key))
		} else {
			assert.Equal(value, req.PostFormValue(key))
		}
	}
}

func TestUploadFile(t *testing.T) {
	assert := assert.New(t)
	aws := S3Stub()
	api := setupTestApi("fake", false)
	defer aws.Close()
	video := Video{Id: VIDEO_ID2, Api: &api}
	valid_file := "video.go"
	err := video.UploadFile(valid_file)
	assert.NotNil(err)
	assert.Equal("Invalid uuid. Example: '1c0e3ea4529011e6991554a050defa20'.", err.Error())
	api.Key = API_KEY
	err = video.UploadFile("myfile.mp4")
	assert.NotNil(err)
	assert.Equal("file 'myfile.mp4' does not exist", err.Error())
	video.UploadInfo.setURL(aws.URL)
	err = video.UploadFile(valid_file)
	assert.Nil(err)
	// use an invalid key and it should return an error
	video.UploadInfo["key"] = "fakekey"
	err = video.UploadFile(valid_file)
	assert.NotNil(err)
	assert.Equal("At least one of the pre-conditions you specified did not hold", err.Error())

}
