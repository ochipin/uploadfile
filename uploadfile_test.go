package uploadfile

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// テスト開始時にテストデータを作成する
func init() {
	os.MkdirAll("test/dst", 0755)
	os.MkdirAll("test/src", 0755)

	// 5 byte
	ioutil.WriteFile("test/src/data1.txt", []byte("abcde"), 0644)
	// 10 byte
	ioutil.WriteFile("test/src/data2.txt", []byte("abcdefghij"), 0644)
	// 15 byte
	ioutil.WriteFile("test/src/data3.txt", []byte("abcdefghijklmno"), 0644)
	// 20 byte
	ioutil.WriteFile("test/src/data4.txt", []byte("abcdefghijklmnopqrst"), 0644)
	// 25 byte
	ioutil.WriteFile("test/src/data5.txt", []byte("abcdefghijklmnopqrstuvwxy"), 0644)
}

func Upload(client *http.Client, url string, values map[string]io.Reader) (err error) {
	var buf bytes.Buffer
	var writer = multipart.NewWriter(&buf)

	for name, r := range values {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}

		if x, ok := r.(*os.File); ok {
			if fw, err = writer.CreateFormFile(name, x.Name()); err != nil {
				panic(err)
			}
		} else {
			if fw, err = writer.CreateFormField(name); err != nil {
				panic(err)
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			panic(err)
		}
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		panic(err)
	}

	// multipart/form-data
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	if res.StatusCode != http.StatusOK {
		return
	}

	return
}

func readFile(filename string) *os.File {
	r, _ := os.Open(filename)
	return r
}

// テスト1: 成功
func Test__UPLOAD_SUCCESS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := New(r, &UploadFiles{
			SaveFile:  "test/dst/%Y%m%d_%H%M%S_%g_%f",
			Perm:      0644,
			MaxSize:   10,
			Filenum:   2,
			Overwrite: true,
			Unique:    true,
		})
		// ParseMultipartForm をコールする前なので、この時点ではデータは取れない
		if data := f.Get("file"); data != nil {
			t.Fatal("New: Error")
		}

		r.ParseMultipartForm(32 << 20)
		f = New(r, &UploadFiles{
			SaveFile:  "test/dst/%Y%m%d_%H%M%S_%g_%f",
			Perm:      0644,
			MaxSize:   10,
			Filenum:   2,
			Overwrite: true,
			Unique:    true,
		})
		if err := f.SumLimitExceeded(); err != nil {
			t.Fatal("SumLimitExceeded: Error")
		}
		if err := f.LimitExceeded(); err != nil {
			t.Fatal("LimitExceeded: Error")
		}
		if err := f.Write(); err != nil {
			t.Fatal(err)
		}

		if data := f.Get("file"); data == nil {
			t.Fatal("Get: Error")
		}

		if data := f.Get("file1"); data != nil {
			t.Fatal("Get: Error")
		}
	}))
	defer ts.Close()

	client := ts.Client()
	values := map[string]io.Reader{
		"file": readFile("test/src/data1.txt"),
	}

	Upload(client, ts.URL, values)
}

// テスト2: 失敗
func Test__UPLOAD_FILENUM_EXCEEDED_ERROR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)
		f := New(r, &UploadFiles{
			SaveFile:  "test/dst/%Y%m%d_%H%M%S_%g_%f",
			Perm:      0644,
			MaxSize:   100,
			Filenum:   2,
			Overwrite: true,
			Unique:    true,
		})
		if err := f.SumLimitExceeded(); err == nil {
			t.Fatal("SumLimitExceeded: Error")
		} else {
			fmt.Println(err)
		}
	}))
	defer ts.Close()

	client := ts.Client()
	values := map[string]io.Reader{
		"file1": readFile("test/src/data1.txt"),
		"file2": readFile("test/src/data2.txt"),
		"file3": readFile("test/src/data3.txt"),
	}

	Upload(client, ts.URL, values)
}

// テスト3: 失敗
func Test__UPLOAD_SUMFILESIZE_EXCEEDED_ERROR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)
		f := New(r, &UploadFiles{
			SaveFile:  "test/dst/%Y%m%d_%H%M%S_%g_%f",
			Perm:      0644,
			MaxSize:   15,
			Filenum:   3,
			Overwrite: true,
			Unique:    true,
		})
		if err := f.SumLimitExceeded(); err == nil {
			t.Fatal("SumLimitExceeded: Error")
		} else {
			fmt.Println(err)
		}
	}))
	defer ts.Close()

	client := ts.Client()
	values := map[string]io.Reader{
		"file1": readFile("test/src/data1.txt"),
		"file2": readFile("test/src/data2.txt"),
		"file3": readFile("test/src/data3.txt"),
	}

	Upload(client, ts.URL, values)
}

// テスト4: 失敗
func Test__UPLOAD_FILESIZE_EXCEEDED_ERROR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)
		f := New(r, &UploadFiles{
			SaveFile:  "test/dst/%Y%m%d_%H%M%S_%g_%f",
			Perm:      0644,
			MaxSize:   10,
			Filenum:   3,
			Overwrite: true,
			Unique:    true,
		})
		if err := f.LimitExceeded(); err == nil {
			t.Fatal("SumLimitExceeded: Error")
		} else {
			fmt.Println(err)
		}
	}))
	defer ts.Close()

	client := ts.Client()
	values := map[string]io.Reader{
		"file1": readFile("test/src/data1.txt"),
		"file2": readFile("test/src/data2.txt"),
		"file3": readFile("test/src/data3.txt"),
	}

	Upload(client, ts.URL, values)
}

// テスト5: 失敗

func Test__UPLOAD_UNIQUE_ERROR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)
		f := New(r, &UploadFiles{
			SaveFile:  "test/dst/%Y%m%d_%H%M%S_%g_%f",
			Perm:      0644,
			MaxSize:   100,
			Filenum:   3,
			Overwrite: true,
			Unique:    true,
		})
		if err := f.SumLimitExceeded(); err == nil {
			t.Fatal("SumLimitExceeded: Error")
		} else {
			fmt.Println(err)
		}
		if err := f.LimitExceeded(); err == nil {
			t.Fatal("SumLimitExceeded: Error")
		} else {
			fmt.Println(err)
		}
	}))
	defer ts.Close()

	client := ts.Client()
	values := map[string]io.Reader{
		"file1": readFile("test/src/data1.txt"),
		"file2": readFile("test/src/data1.txt"),
		"file3": readFile("test/src/data3.txt"),
	}

	Upload(client, ts.URL, values)
}

// テスト6: 失敗

func Test__UPLOAD_OVERWRITE_ERROR(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)
		f := New(r, &UploadFiles{
			SaveFile:  "test/dst/duplicate.txt",
			Perm:      0644,
			MaxSize:   100,
			Filenum:   3,
			Overwrite: false,
			Unique:    true,
		})
		if err := f.Write(); err == nil {
			t.Fatal("Write: Error")
		}
	}))
	defer ts.Close()

	client := ts.Client()
	values := map[string]io.Reader{
		"file1": readFile("test/src/data1.txt"),
		"file2": readFile("test/src/data1.txt"),
		"file3": readFile("test/src/data3.txt"),
	}

	Upload(client, ts.URL, values)
}
