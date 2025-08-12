package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	scrapingURL     = "https://www.trustpilot.com/review/%s"
	scrapingPageURL = "https://www.trustpilot.com/review/%s?languages=all&page=%d"
)

var (
	ErrPageNotFound = errors.New("page not found")
)

type Review struct {
	ID           string `json:"id"`
	Content      string `json:"content"`
	Date         string `json:"date"`
	Rating       string `json:"rating"`
	Link         string `json:"link"`
	ReviewerName string `json:"reviewer_name"`
}

type CompanyReviews struct {
	Reviews []*Review `json:"reviews"`
}

func main() {
	var productName string
	flag.StringVar(&productName, "product", "", "Product name to scrape reviews for")
	flag.Parse()

	if productName == "" {
		log.Fatal("Please provide a product name to scrape reviews for")
	}
	log.Printf("Scraping reviews for %s", productName)
	log.Printf("Start scraping reviews for %s", productName)
	out := make(chan struct{})
	reviews := make(chan *Review)

	productReviews := &CompanyReviews{}
	productReviews.Reviews = make([]*Review, 0)

	go getProductReviews(productName, reviews, out)

	go func() {
		for review := range reviews {
			if review == nil {
				break
			}

			productReviews.Reviews = append(productReviews.Reviews, review)
		}

		close(out)
	}()

	<-out

	// Save reviews to JSON file

	jsonFile, err := os.Create(fmt.Sprintf("trustpilot_reviews_%s.json", productName))
	if err != nil {
		log.Fatal(err)
	}
	defer jsonFile.Close()

	jsonEncoder := json.NewEncoder(jsonFile)
	err = jsonEncoder.Encode(productReviews)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Successfully scraped %d reviews for %s", len(productReviews.Reviews), productName)
}

func getProductReviews(productName string, reviews chan<- *Review, done chan<- struct{}) {
	defer close(reviews)

	pageNumber := 0
	for {
		pageNumber++
		if err := getPageProductReviews(productName, pageNumber, reviews); err != nil {
			if err == ErrPageNotFound {
				log.Printf("reached last page %d\n", pageNumber)
			}

			done <- struct{}{}
			break
		}
	}
}

func getPageProductReviews(productName string, pageNumber int, reviews chan<- *Review) error {
	productURL := fmt.Sprintf(scrapingPageURL, productName, pageNumber)
	log.Println("Scraping page", pageNumber, "of", productURL)

	res, err := http.Get(productURL)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return ErrPageNotFound
	}

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
	}

	doc.Find("div[class^='styles_cardWrapper__']:has(article)").Each(extractReviewFunc(reviews, productURL))

	return nil
}

func extractReviewFunc(reviews chan<- *Review, productURL string) func(i int, s *goquery.Selection) {
	return func(i int, s *goquery.Selection) {
		reviewerName := s.Find("aside[class^='styles_consumerInfoWrapper__'] a[name='consumer-profile'] > span[class^='typography_heading']").First().Text()

		createdAtTime := ""
		ok := false

		if createdAtTime, ok = s.Find("time").First().Attr("datetime"); !ok {
			log.Println("Could not find createdAtTime", reviewerName)
		}

		// narrow the serch to the review content section
		s = s.Find("section[class^='styles_reviewContentwrapper__']")

		rating, ok := s.Find("div[class^='styles_reviewHeader__']").First().Attr("data-service-review-rating")
		if !ok {
			log.Println("Could not find rating", reviewerName)
		}

		linkToReview := ""
		if linkToReview, ok = s.Find("div[class^='styles_reviewContent__'] a").First().Attr("href"); !ok {
			log.Println("Could not find linkToReview", reviewerName)
		} else {
			linkToReview = productURL + linkToReview
		}

		reviewContent := s.Find("div[class^='styles_reviewContent__'] p[data-service-review-text-typography='true']").First()

		reviewContent.RemoveAttr("class")
		htmlContent := reviewContent.Text()

		//remove all <br> tags and "\"" characters
		htmlContent = strings.TrimPrefix(htmlContent, "\"")
		htmlContent = strings.TrimPrefix(htmlContent, "<br>")
		htmlContent = strings.TrimSuffix(htmlContent, "\"")
		htmlContent = strings.TrimSuffix(htmlContent, "\n")

		reviews <- &Review{
			ID:           fmt.Sprintf("review-%d", i+1),
			Content:      htmlContent,
			Date:         createdAtTime,
			Rating:       rating,
			Link:         linkToReview,
			ReviewerName: reviewerName,
		}
	}
}
