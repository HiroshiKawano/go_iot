// デバイスAPI用 Bearer トークンを発行するCLIコマンド。
// 指定したユーザーに対してトークンを生成し、DB にハッシュを保存、
// 平文トークンを標準出力に表示する (以降再表示不可)。
//
// 使い方:
//
//	make gen-token user=1 name="ハウスA温湿度計"
//	# または
//	go run ./cmd/gen-token -user=1 -name="ハウスA温湿度計"
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/infra/token"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

func main() {
	var (
		userID    int64
		name      string
		abilities string
		expireDay int
	)
	flag.Int64Var(&userID, "user", 0, "トークンを発行する user_id (必須)")
	flag.StringVar(&name, "name", "", "トークン名 (通常はデバイス名と揃える)")
	flag.StringVar(&abilities, "abilities", `["sensor:write"]`, "権限一覧 (JSON配列)")
	flag.IntVar(&expireDay, "expire-days", 0, "有効期間(日)。0 = 無期限")
	flag.Parse()

	if userID == 0 || name == "" {
		fmt.Fprintln(flag.CommandLine.Output(), "使い方: gen-token -user=<user_id> -name=<トークン名> [-abilities=<JSON>] [-expire-days=<N>]")
		flag.PrintDefaults()
		log.Fatal("-user と -name は必須です")
	}

	var abilitiesRaw json.RawMessage
	if err := json.Unmarshal([]byte(abilities), &abilitiesRaw); err != nil {
		log.Fatalf("-abilities はJSON配列として解釈できません: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := infradb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	q := repository.New(pool)

	plaintext, hash, err := token.Generate()
	if err != nil {
		log.Fatal(err)
	}

	var expiresAt pgtype.Timestamptz
	if expireDay > 0 {
		expiresAt = pgtype.Timestamptz{
			Time:  time.Now().Add(time.Duration(expireDay) * 24 * time.Hour),
			Valid: true,
		}
	}

	tok, err := q.CreateDeviceToken(ctx, repository.CreateDeviceTokenParams{
		UserID:    userID,
		Name:      name,
		TokenHash: hash,
		Abilities: abilitiesRaw,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		log.Fatalf("トークン保存失敗: %v", err)
	}

	fmt.Println("✅ トークン発行成功 (⚠️ 平文トークンは今回のみ表示されます)")
	fmt.Println()
	fmt.Println("  token_id:    ", tok.ID)
	fmt.Println("  user_id:     ", tok.UserID)
	fmt.Println("  name:        ", tok.Name)
	fmt.Println("  abilities:   ", abilities)
	if tok.ExpiresAt.Valid {
		fmt.Println("  expires_at:  ", tok.ExpiresAt.Time.Format(time.RFC3339))
	} else {
		fmt.Println("  expires_at:  無期限")
	}
	fmt.Println()
	fmt.Println("  🔑 平文トークン (ESP8266 に設定):")
	fmt.Println("     ", plaintext)
	fmt.Println()
	fmt.Println("  HTTP ヘッダ例:")
	fmt.Printf("     Authorization: Bearer %s\n", plaintext)
}
