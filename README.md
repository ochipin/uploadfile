アップロードファイル管理ライブラリ
===

アップロードされたファイルを操作するライブラリ。
使用方法を次のサンプルで記す。

```go
package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/ochipin/uploadfile"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // アップロードされたファイル情報を取得する
        f := uploadfile.New(r, &uploadfile.UploadFiles{
            // ファイル名の保存はフォーマット指定子を利用可能
            // %Y: 2018(YYYY)
            // %m: 08(mm)
            // %d: 09(dd)
            // %H: 23(HH)
            // %M: 17(MM)
            // %S: 08(SS)
            // %g: MD5
            // %f: アップロードされたファイル名
            SaveFile:  "files/%Y/%m/%y%m%d_%H%M%S_%g_%f",
            // Byte単位で指定する。下記例は、 10 << 10 なので、10KB単位となる
            MaxSize:   10 << 10,
            // ファイル保存時のパーミッション
            Perm:      0644,
            // 同名のファイルの上書きを許可
            // true:  上書きを許可する
            // false: 上書きを許可しない
            Overwrite: true,
            // 一度にファイルアップロードできるファイル数の上限値。0以下は無制限となる
            Filenum:   3,
            // 重複したファイル名のアップロードの許可設定
            // true:  重複ファイル名を許可しない
            // false: 重複ファイル名を許可する
            Unique:    true,
        })

        // アップロードされたファイル数をカウントする
        fmt.Println(f.Filenum())

        // アップロードされたすべてのファイルの合計サイズを取得する
        fmt.Println(f.Size())

        // アップロードされた、全ファイルのエラーチェック
        if err := f.SumLimitExceeded(); err != nil {
            log.Println(err)
            return
        }

        // アップロードされた、ファイル単体のエラーチェック
        if err := f.LimitExceeded(); err != nil {
            log.Println(err)
            return
        }
    
        // アップロードされた全ファイルを保存する
        f.Write()

        // ファイル単体の情報を取得する
        get := f.Get("upload")
        // 取得したファイル単体を保存する
        // 保存時は、ファイル名、パーミッション、上書き設定の3つを引数に渡す必要がある。
        get.Write("filename", 0644, false)

        // ループによる、全ファイルの処理
        for name, headers := range f.Files() {
            // <input type='file' name='upload' ...
            // upload を出力する
            fmt.Println(name)
            for _, header := range headers {
                // ファイル保存
                header.SaveFile("%Y%m_filename", 0644, true)
                fmt.Println(header.Filename) // ファイル名
                fmt.Println(header.Size)     // ファイルサイズ
                fmt.Println(header.Header)   // ヘッダ情報
                // io.Reader と組み合わせることで、メールにアップロードされたファイルも添付可能
                var r io.Reader
                r = header
            }
        }
    })
    http.ListenAndServe(":8080", nil)
}
```
