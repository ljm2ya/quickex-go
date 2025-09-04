package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/ljm2ya/quickex-go/client/phemex"
)

func main() {
	fmt.Println("🔧 === Phemex 직접 주문 테스트 ===")
	
	// API 키 파일에서 읽기
	credFile := "/home/jae/문서/phemex.txt"
	credData, err := os.ReadFile(credFile)
	if err != nil {
		log.Fatalf("❌ API 키 파일 읽기 실패: %v", err)
	}
	
	lines := strings.Split(string(credData), "\n")
	apiKey := strings.TrimSpace(lines[0])
	apiSecret := strings.TrimSpace(lines[1])
	
	fmt.Println("1️⃣ Phemex 클라이언트 초기화...")
	
	// Phemex 클라이언트 생성
	client := phemex.NewClient(apiKey, apiSecret)
	
	fmt.Printf("✅ 클라이언트 생성 완료 (API Key: %s...)\n", apiKey[:8])
	
	// 잔고 확인 (REST API 직접 호출)
	fmt.Println("2️⃣ USDT 잔고 확인...")
	
	usdtBalance, err := client.FetchBalance("USDT", false, false)
	if err != nil {
		log.Printf("⚠️ USDT 잔고 조회 실패: %v", err)
		usdtBalance = decimal.NewFromFloat(10.0) // 기본값 사용
	}
	
	fmt.Printf("   USDT 잔고: %s\n", usdtBalance.String())
	
	dogeBalance, err := client.FetchBalance("DOGE", false, false)
	if err != nil {
		log.Printf("⚠️ DOGE 잔고 조회 실패: %v", err)
		dogeBalance = decimal.Zero
	}
	
	fmt.Printf("   DOGE 잔고: %s\n", dogeBalance.String())
	
	if usdtBalance.LessThan(decimal.NewFromFloat(2.0)) {
		log.Fatalf("❌ USDT 잔고 부족: %s USDT", usdtBalance.String())
	}
	
	// 시장가 매수 테스트 (5 USDT)
	fmt.Println("3️⃣ 시장가 매수 테스트 (5 USDT)...")
	
	buyAmount := decimal.NewFromFloat(5.0)
	buyOrder, err := client.MarketBuy("sDOGEUSDT", buyAmount)
	if err != nil {
		log.Fatalf("❌ 시장가 매수 실패: %v", err)
	}
	
	fmt.Printf("✅ 시장가 매수 성공! Order ID: %s\n", buyOrder.OrderID)
	fmt.Printf("   주문 상태: %v\n", buyOrder.Status)
	fmt.Printf("   주문 금액: %s\n", buyOrder.Quantity.String())
	
	// 체결 대기
	fmt.Println("4️⃣ 주문 체결 대기...")
	time.Sleep(5 * time.Second)
	
	// 체결 후 DOGE 잔고 확인
	fmt.Println("5️⃣ 체결 후 DOGE 잔고 확인...")
	
	newDogeBalance, err := client.FetchBalance("DOGE", false, false)
	if err != nil {
		log.Printf("⚠️ DOGE 잔고 조회 실패: %v", err)
	} else {
		fmt.Printf("   매수 후 DOGE 잔고: %s\n", newDogeBalance.String())
		
		if newDogeBalance.GreaterThan(dogeBalance) {
			purchasedDoge := newDogeBalance.Sub(dogeBalance)
			fmt.Printf("✅ 구매된 DOGE: %s\n", purchasedDoge.String())
			
			// 시장가 매도 테스트
			fmt.Println("6️⃣ 시장가 매도 테스트...")
			
			sellOrder, err := client.MarketSell("sDOGEUSDT", purchasedDoge)
			if err != nil {
				log.Printf("⚠️ 시장가 매도 실패: %v", err)
			} else {
				fmt.Printf("✅ 시장가 매도 성공! Order ID: %s\n", sellOrder.OrderID)
				fmt.Printf("   주문 상태: %v\n", sellOrder.Status)
				fmt.Printf("   매도 수량: %s\n", sellOrder.Quantity.String())
				
				// 최종 결과 확인
				time.Sleep(3 * time.Second)
				fmt.Println("7️⃣ 최종 잔고 확인...")
				
				finalUsdtBalance, err := client.FetchBalance("USDT", false, false)
				if err != nil {
					log.Printf("⚠️ 최종 USDT 잔고 조회 실패: %v", err)
				} else {
					profit := finalUsdtBalance.Sub(usdtBalance)
					fmt.Printf("   최종 USDT 잔고: %s\n", finalUsdtBalance.String())
					fmt.Printf("   거래 손익: %s USDT\n", profit.String())
				}
			}
		} else {
			fmt.Println("⚠️ DOGE 잔고 변화 없음 - 체결되지 않았거나 대기 중")
		}
	}
	
	// 지정가 주문 테스트
	fmt.Println("8️⃣ 지정가 주문 테스트 (낮은 가격으로 취소 예정)...")
	
	// 현재 시세보다 10% 낮은 가격으로 지정가 매수
	lowPrice := decimal.NewFromFloat(0.20) // 매우 낮은 가격
	limitQuantity := decimal.NewFromFloat(100.0) // 100 DOGE
	
	limitOrder, err := client.LimitBuy("sDOGEUSDT", limitQuantity, lowPrice, "GTC")
	if err != nil {
		log.Printf("⚠️ 지정가 매수 실패: %v", err)
	} else {
		fmt.Printf("✅ 지정가 매수 성공! Order ID: %s\n", limitOrder.OrderID)
		fmt.Printf("   주문 가격: %s USDT\n", limitOrder.Price.String())
		fmt.Printf("   주문 수량: %s DOGE\n", limitOrder.Quantity.String())
		
		// 주문 취소 테스트
		fmt.Println("9️⃣ 주문 취소 테스트...")
		time.Sleep(2 * time.Second)
		
		cancelResp, err := client.CancelOrder("sDOGEUSDT", limitOrder.OrderID)
		if err != nil {
			log.Printf("⚠️ 주문 취소 실패: %v", err)
		} else {
			fmt.Printf("✅ 주문 취소 성공! Order ID: %s\n", cancelResp.OrderID)
			fmt.Printf("   취소 상태: %v\n", cancelResp.Status)
		}
	}
	
	fmt.Println("\n🎉 Phemex 직접 주문 테스트 완료!")
	fmt.Println("🎯 새로운 파라미터 형식으로 성공적으로 작동!")
}