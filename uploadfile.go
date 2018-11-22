package uploadfile

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UploadFiles : ファイルアップロードの設定
type UploadFiles struct {
	SaveFile  string      // 保存するファイルのパス。 ex) files/%Y/%m/%y%m%d%H%M%D_%g
	MaxSize   int64       // アップロードするファイルのサイズ制限(Byte単位)。ex) 20 * 1024 * 1024 20MB
	Perm      os.FileMode // 保存するファイルのパーミッション。 ex) 0644
	Overwrite bool        // true: 上書き, false: 上書きしない
	Unique    bool        // true: 同名のファイルは除外, false: 同名のファイルを除外しない
	Filenum   int         // 1度のリクエストで送信できるファイル数の上限値。0は無制限。
}

// New : アップロードされたファイル情報を元に File 構造体を生成する
func New(r *http.Request, fileinfo *UploadFiles) *File {
	var filenum = 0
	if fileinfo.Filenum > 0 {
		filenum = fileinfo.Filenum
	}
	var file = &File{
		headers:   make(map[string][]*FileHeader),
		perm:      fileinfo.Perm,
		savefile:  fileinfo.SaveFile,
		overwrite: fileinfo.Overwrite,
		maxsize:   fileinfo.MaxSize,
		filenum:   filenum,
	}

	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return file
	}

	var uniq = make(map[string]bool)
	for name, headers := range r.MultipartForm.File {
		var fileheaders []*FileHeader
		for _, header := range headers {
			// Unique = true の場合、同名のファイルは除外する
			if fileinfo.Unique {
				if _, ok := uniq[header.Filename]; ok {
					if file.unique == nil {
						file.unique = make(map[string]bool)
					}
					file.unique[header.Filename] = true
					continue
				}
				uniq[header.Filename] = true
			}

			fileheaders = append(fileheaders, &FileHeader{header})
		}
		file.headers[name] = fileheaders
	}

	return file
}

const (
	// NumLimitExceeded : ファイルアップロード数の上限値に到達した
	NumLimitExceeded = 1
	// SumLimitExceeded : アップロードしたファイルの合計サイズが上限値に到達した
	SumLimitExceeded = 2
	// LimitExceeded : アップロードしたファイルサイズが上限値に到達した
	LimitExceeded = 3
	// UniqueFile : 重複したファイルをアップロードされた
	UniqueFile = 4
)

// Error : エラー管理構造体
type Error struct {
	Maxsize   int64                // アップロードできる最大のファイルサイズ数(Byte単位)
	Filename  string               // アップロードファイル名
	Filesize  int64                // アップロードしたファイルのサイズ(Byte単位)
	Attrname  string               // HTML上に記載されている属性名
	Header    textproto.MIMEHeader // ヘッダ情報
	ErrorType int                  // エラーの種類
}

func (e *Error) Error() (result string) {
	switch e.ErrorType {
	case NumLimitExceeded:
		result = fmt.Sprintf("payload too large file. upload file num [%d > maxnum(%d)]", e.Filesize, e.Maxsize)
	case SumLimitExceeded:
		result = fmt.Sprintf("payload too large file. all upload file sum size [%d > max(%d bytes)]", e.Filesize, e.Maxsize)
	case LimitExceeded:
		result = fmt.Sprintf("payload too large file. '%s' = [%s(%d bytes) > max(%d bytes)]", e.Attrname, e.Filename, e.Filesize, e.Maxsize)
	case UniqueFile:
		result = fmt.Sprintf("payload too large file. '%s' is duplicate", e.Filename)
	}
	return
}

// File : アップロードファイル管理構造体
type File struct {
	headers   map[string][]*FileHeader
	perm      os.FileMode     // 保存するファイルのパーミッション
	savefile  string          // 保存するファイル名
	overwrite bool            // 保存時に、ファイルが存在する場合同名のファイルを上書きするか否か(true:上書き, false:上書きしない)
	maxsize   int64           // 保存するファイル名のファイルサイズ制限値
	unique    map[string]bool // 同名のファイルが存在した場合,同名のファイル名が格納される
	filenum   int             // アップロードファイル数の上限値が格納される
}

// Filenum : アップロードされたファイル数をカウント
func (file *File) Filenum() int {
	var filenum int
	for _, headers := range file.headers {
		filenum += len(headers)
	}
	return filenum
}

func (file *File) Write() error {
	for name, headers := range file.Files() {
		for _, header := range headers {
			if err := header.Write(file.savefile, file.perm, file.overwrite); err != nil {
				return fmt.Errorf("%s: %s", name, err.Error())
			}
		}
	}
	return nil
}

// Size : アップロードされたファイルすべての合計サイズを返却する
func (file *File) Size() int64 {
	var result int64
	for _, headers := range file.headers {
		for _, header := range headers {
			result += header.Size
		}
	}
	return result
}

// Files : アップロードされたファイルすべての情報を取得する
func (file *File) Files() map[string][]*FileHeader {
	return file.headers
}

// Headers : 指定したキー名(name属性名)に一致するファイル一覧の情報を取得する
func (file *File) Headers(name string) []*FileHeader {
	header, _ := file.headers[name]
	return header
}

// Get : 指定したキー名(name属性名)に一致するファイル1つの情報を取得する
func (file *File) Get(name string) *FileHeader {
	headers := file.Headers(name)
	if len(headers) == 0 {
		return nil
	}
	return headers[0]
}

// SumLimitExceeded : アップロードされたファイルの総合計サイズが、MaxSizeを超過した場合エラーを返却する
func (file *File) SumLimitExceeded() error {
	if file.Size() > file.maxsize {
		return &Error{
			Maxsize:   file.maxsize,
			Filesize:  file.Size(),
			ErrorType: SumLimitExceeded,
		}
	}

	for name, _ := range file.unique {
		return &Error{
			Filename:  name,
			ErrorType: UniqueFile,
		}
	}

	return file.numExceeded()
}

// LimitExceeded : アップロードされたファイルのどれか1つがMaxSizeを超過した場合エラーを返却する
func (file *File) LimitExceeded() error {
	for name, headers := range file.headers {
		for _, header := range headers {
			if file.maxsize != 0 && file.maxsize < header.Size {
				return &Error{
					Maxsize:   file.maxsize,
					Filename:  header.Filename,
					Filesize:  header.Size,
					Attrname:  name,
					Header:    header.Header,
					ErrorType: LimitExceeded,
				}
			}
		}
	}

	for name, _ := range file.unique {
		return &Error{
			Filename:  name,
			ErrorType: UniqueFile,
		}
	}

	return file.numExceeded()
}

// アップロードされたファイル数が上限値を超過していた場合、エラーを返却する
func (file *File) numExceeded() error {
	num := file.Filenum()
	if file.filenum != 0 && num > file.filenum {
		return &Error{
			Maxsize:   int64(file.filenum),
			Filesize:  int64(num),
			ErrorType: NumLimitExceeded,
		}
	}
	return nil
}

// FileHeader : アップロードされたファイル情報を管理する構造体
type FileHeader struct {
	*multipart.FileHeader
}

func (header *FileHeader) Write(format string, perm os.FileMode, overwrite bool) error {
	// ファイルオープン
	file, err := header.Open()
	if err != nil {
		return err
	}
	// 関数復帰後、ファイルクローズ
	defer file.Close()

	// ファイル内容を読み取り
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	filename := header.Filename
	if format != "" {
		now := time.Now()
		year := fmt.Sprint(now.Year())
		hash := md5.New()
		io.WriteString(hash, fmt.Sprintf("%s%d", header.Filename, header.Size))
		rep := strings.NewReplacer(
			"%Y", year,
			"%y", year[2:],
			"%m", fmt.Sprintf("%02d", int(now.Month())),
			"%d", fmt.Sprintf("%02d", now.Day()),
			"%H", fmt.Sprintf("%02d", now.Hour()),
			"%M", fmt.Sprintf("%02d", now.Minute()),
			"%S", fmt.Sprintf("%02d", now.Second()),
			"%f", header.Filename,
			"%g", fmt.Sprintf("%x", hash.Sum(nil)),
		)
		filename = rep.Replace(format)
	}

	if dir, _ := filepath.Split(filename); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	// 保存するファイルと同名のファイルが存在するか否かをチェック
	if !overwrite {
		// 同名のファイルが存在した場合、ファイルの上書きはしない
		if _, err := os.Stat(filename); err == nil {
			return fmt.Errorf("'%s' is already exists", filename)
		}
	}

	// 読み取った内容を保存する
	return ioutil.WriteFile(filename, buf, perm)
}
