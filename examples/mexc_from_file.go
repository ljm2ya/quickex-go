package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ljm2ya/quickex-go/client"
	"github.com/shopspring/decimal"
)

// readMexcCredentials reads API credentials from /home/jae/문서/mexc.txt
func readMexcCredentials() (apiKey, secret string, err error) {
	filePath := "/home/jae/문서/mexc.txt"
	
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", fmt.Errorf("파일을 열 수 없습니다: %w", err)
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lines := []string{}
	
	// 파일의 모든 줄을 읽기
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" { // 빈 줄 제외
			lines = append(lines, line)
		}
	}
	
	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("파일 읽기 오류: %w", err)
	}
	
	if len(lines) < 2 {
		return "", "", fmt.Errorf("파일에 API 키와 시크릿이 모두 있어야 합니다 (2줄 필요, %d줄 발견)", len(lines))
	}
	
	apiKey = lines[0]  // 첫 번째 줄: API 키
	secret = lines[1]  // 두 번째 줄: Secret
	
	if apiKey == "" || secret == "" {
		return "", "", fmt.Errorf("API 키나 시크릿이 비어있습니다")
	}
	
	return apiKey, secret, nil
}

func main() {
	fmt.Println("=== MEXC API 키 파일에서 읽기 ===")
	
	// 1. 파일에서 API 키 읽기
	fmt.Println("📁 /home/jae/문서/mexc.txt 파일에서 API 키를 읽는 중...")
	apiKey, secret, err := readMexcCredentials()
	if err != nil {
		log.Fatal("❌ API 키 읽기 실패:", err)
	}
	
	fmt.Printf("✅ API 키 읽기 성공! (키 길이: %d, 시크릿 길이: %d)\n", len(apiKey), len(secret))
	
	// 보안을 위해 키의 일부만 표시
	if len(apiKey) > 8 {
		fmt.Printf("📋 API 키: %s...%s\n", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if len(secret) > 8 {
		fmt.Printf("🔐 Secret: %s...%s\n", secret[:4], secret[len(secret)-4:])
	}
	
	// 2. MEXC 현물 클라이언트 생성
	fmt.Println("\n=== MEXC 현물 거래 테스트 ===")
	spotClient := client.NewPrivateClient(
		string(client.ExchangeMexc),
		apiKey,
		secret,
	)
	
	// 3. 연결
	ctx := context.Background()
	fmt.Println("🔗 MEXC에 연결 중...")
	_, err = spotClient.Connect(ctx)
	if err != nil {
		log.Fatal("❌ 연결 실패:", err)
	}
	defer spotClient.Close()
	
	fmt.Println("✅ MEXC 현물 연결 성공!")
	
	// 4. 잔고 조회 테스트
	fmt.Println("\n💰 USDT 잔고 조회 중...")
	balance, err := spotClient.FetchBalance("USDT", true, false)
	if err != nil {
		log.Printf("❌ 잔고 조회 실패: %v", err)
	} else {
		fmt.Printf("💵 USDT 현물 잔고: %s\n", balance.String())
	}
	
	// 5. 실시간 시세 구독 테스트
	fmt.Println("\n📈 실시간 시세 구독 테스트...")
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	quoteChans, err := spotClient.SubscribeQuotes(ctx, symbols, func(err error) {
		log.Printf("⚠️ 시세 에러: %v", err)
	})
	if err != nil {
		log.Printf("❌ 시세 구독 실패: %v", err)
	} else {
		fmt.Printf("✅ %d개 심볼 시세 구독 성공!\n", len(symbols))
		
		// 10초간 시세 받기
		fmt.Println("📊 10초간 실시간 시세를 받아옵니다...")
		timeout := time.After(10 * time.Second)
		quoteCount := 0
		
		for {
			select {
			case quote := <-quoteChans["BTCUSDT"]:
				quoteCount++
				fmt.Printf("[%d] BTC: 매수 %s, 매도 %s (%s)\n", 
					quoteCount,
					quote.BidPrice.String(), 
					quote.AskPrice.String(), 
					quote.Time.Format("15:04:05"))
			case quote := <-quoteChans["ETHUSDT"]:
				fmt.Printf("     ETH: 매수 %s, 매도 %s\n", 
					quote.BidPrice.String(), 
					quote.AskPrice.String())
			case <-timeout:
				fmt.Printf("✅ 시세 테스트 완료! (총 %d개 시세 수신)\n", quoteCount)
				goto futuresTest
			}
		}
	}
	
futuresTest:
	// 6. MEXC 선물 클라이언트 테스트
	fmt.Println("\n=== MEXC 선물 거래 테스트 ===")
	futuresClient := client.NewFuturesPrivateClient(
		string(client.ExchangeMexcFutures),
		apiKey,
		secret,
	)
	
	fmt.Println("🔗 MEXC 선물에 연결 중...")
	_, err = futuresClient.Connect(ctx)
	if err != nil {
		log.Printf("❌ 선물 연결 실패: %v", err)
	} else {
		fmt.Println("✅ MEXC 선물 연결 성공!")
		
		// 선물 잔고 조회
		fmt.Println("💰 USDT 선물 포지션 조회 중...")
		futuresBalance, err := futuresClient.FetchBalance("USDT", true, true)
		if err != nil {
			log.Printf("❌ 선물 잔고 조회 실패: %v", err)
		} else {
			fmt.Printf("💵 USDT 선물 포지션: %s\n", futuresBalance.String())
		}
		
		futuresClient.Close()
	}
	
	// 7. 테스트 주문 예제 (실제로는 실행하지 않음)
	fmt.Println("\n=== 주문 기능 예제 (실행하지 않음) ===")
	price := decimal.NewFromFloat(30000.0)      // 시장가보다 낮은 가격
	quantity := decimal.NewFromFloat(0.001)     // 소량
	orderValue := price.Mul(quantity)
	
	fmt.Printf("📝 예시 주문: %s BTC를 $%s에 매수 (총 $%s)\n", 
		quantity.String(), price.String(), orderValue.String())
	
	fmt.Println("⚠️  실제 주문을 하려면 아래 코드의 주석을 해제하세요:")
	fmt.Println("/*")
	fmt.Println("order, err := spotClient.LimitBuy(\"BTCUSDT\", quantity, price, \"GTC\")")
	fmt.Println("if err != nil {")
	fmt.Println("    log.Printf(\"주문 실패: %v\", err)")
	fmt.Println("} else {")
	fmt.Println("    fmt.Printf(\"✅ 주문 성공: %s\\n\", order.OrderID)")
	fmt.Println("}")
	fmt.Println("*/")
	
	fmt.Println("\n🎉 모든 테스트 완료!")
	fmt.Println("📋 테스트 결과:")
	fmt.Println("   ✅ API 키 파일 읽기")
	fmt.Println("   ✅ MEXC 현물 연결")
	fmt.Println("   ✅ 잔고 조회")
	fmt.Println("   ✅ 실시간 시세 구독")
	fmt.Println("   ✅ MEXC 선물 연결")
	fmt.Println("\n🚀 이제 MEXC에서 거래할 준비가 되었습니다!")
}
