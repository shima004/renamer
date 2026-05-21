# renamer

正規表現を使ってファイル・ディレクトリをリネームする CLI ツール。

## インストール

```bash
go install github.com/shimapaca/renamer@latest
```

またはソースからビルド：

```bash
git clone https://github.com/shimapaca/renamer
cd renamer
go build -o renamer .
```

## 使い方

```bash
renamer [-r] [-n] <pattern> <replacement> [directory]
```

| 引数          | 説明                                                        |
| ------------- | ----------------------------------------------------------- |
| `pattern`     | マッチさせる RE2 正規表現（ファイル名に対して適用）         |
| `replacement` | 置換後の文字列。キャプチャグループは `$1`, `$2`, ... で参照 |
| `directory`   | 対象ディレクトリ（省略時はカレントディレクトリ）            |

| フラグ | 説明                                                 |
| ------ | ---------------------------------------------------- |
| `-r`   | サブディレクトリを再帰的に処理する                   |
| `-n`   | ドライラン：プレビューのみ表示し、確認・実行はしない |

## 動作

1. pattern にマッチするファイル・ディレクトリ名を検索
2. リネーム結果をプレビュー表示
3. `y` を入力すると実行、それ以外は中断

```bash
Preview:
  ./001_foo.txt → ./foo_001.txt
  ./002_bar.txt → ./bar_002.txt
Proceed? [y/N]:
```

## 例

### 連番とファイル名を入れ替える

```bash
renamer '(\d+)_(.+)\.txt' '$2_$1.txt' ./files/
```

```bash
Preview:
  files/001_foo.txt → files/foo_001.txt
  files/002_bar.txt → files/bar_002.txt
Proceed? [y/N]: y
Renamed: files/001_foo.txt → files/foo_001.txt
Renamed: files/002_bar.txt → files/bar_002.txt
```

### 拡張子を一括変換（ドライラン）

```bash
renamer -n '\.jpeg$' '.jpg' ./images/
```

### プレフィックスを削除（再帰）

```bash
renamer -r '^tmp_' '' ./data/
```

### スペースをアンダースコアに置換

```bash
renamer ' ' '_'
```

## キャプチャグループの参照

`$1`, `$2`, ... でキャプチャグループを参照できます。`$1_suffix` のように直後に `_` などが続く場合も正しく処理されます（内部的に `${1}` 形式に変換）。

```bash
# $2_$1 → 問題なく動作する
renamer '([a-z]+)_([0-9]+)' '$2_$1'
```

## 注意事項

- パターンはファイル名（basename）に対してマッチします。パスは含みません
- `-r` で再帰処理する場合、深いパスから順にリネームするため、ディレクトリをリネームしても子エントリのパスが無効になりません
- 同名のファイルが既に存在する場合は OS のエラーになります
