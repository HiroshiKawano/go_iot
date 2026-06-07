// Package page は各画面のフルページ templ コンポーネントを提供する。
// ハンドラはここで定義する View 構造体を組み立てて各ページを描画する
// (binding 構造体は handler 側に置き、view → handler の循環依存を避ける)。
package page

// LoginView はログイン画面の描画に必要なデータ。
// Email は再描画時の入力値再表示用。Errors は field 名 → 日本語メッセージ
// ("form" キーはフォーム全体に対する汎用エラー = 認証失敗の共通メッセージ)。
type LoginView struct {
	CSSURL    string
	CSRFToken string
	Email     string
	Errors    map[string]string
}

// RegisterView はユーザー登録画面の描画に必要なデータ。
type RegisterView struct {
	CSSURL    string
	CSRFToken string
	Name      string
	Email     string
	Errors    map[string]string
}
