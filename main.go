package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

const (
	scrapingURL     = "https://www.trustpilot.com/review/%s"
	scrapingPageURL = "https://www.trustpilot.com/review/%s?page=%d"
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

	lastPage := 1
	var err error
	if lastPage, err = getPageProductReviews(productName, lastPage, reviews); err != nil {
		done <- struct{}{}
		return
	}

	// Get all reviews from all pages
	wg := sync.WaitGroup{}
	for i := 2; i <= lastPage; i++ {
		wg.Add(1)
		go func(pageNumber int) {
			defer wg.Done()
			if _, err := getPageProductReviews(productName, pageNumber, reviews); err != nil {
				done <- struct{}{}
				return
			}
		}(i)
	}

	wg.Wait()
}

func getPageProductReviews(productName string, pageNumber int, reviews chan<- *Review) (int, error) {
	productURL := fmt.Sprintf(scrapingPageURL, productName, pageNumber)
	log.Println("Scraping page", pageNumber, "of", productURL)

	res, err := http.Get(productURL)
	if err != nil {
		return pageNumber, err
	}

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return pageNumber, err
	}

	doc.Find("article > div[class^='styles_reviewCardInner__']").Each(extractReviewFunc(reviews, productURL))

	if pageNumber == 1 {
		pn := doc.Find("nav a[name='pagination-button-next']").Prev().Find("span").Text()

		updatedLastPage, err := strconv.Atoi(pn)
		if err != nil {
			log.Println("Could not find last page", err)
			return pageNumber, err
		}

		log.Println("Last page is", updatedLastPage)
		return updatedLastPage, err
	}

	return pageNumber, nil
}

func extractReviewFunc(reviews chan<- *Review, productURL string) func(i int, s *goquery.Selection) {
	return func(i int, s *goquery.Selection) {
		reviewerName := s.Find("aside[class^='styles_consumerInfoWrapper__'] a[name='consumer-profile'] > span[class^='typography_heading-xxs']").First().Text()

		// narrow the serch to the review content section
		s = s.Find("section[class^='styles_reviewContentwrapper__']")

		rating, ok := s.Find("div[class^='styles_reviewHeader__']").First().Attr("data-service-review-rating")
		if !ok {
			log.Println("Could not find rating", reviewerName)
		}

		createdAtTime := ""
		if createdAtTime, ok = s.Find("time").First().Attr("datetime"); !ok {
			log.Println("Could not find createdAtTime", reviewerName)
		}

		linkToReview := ""
		if linkToReview, ok = s.Find("div[class^='styles_reviewContent__'] a").First().Attr("href"); !ok {
			log.Println("Could not find linkToReview", reviewerName)
		} else {
			linkToReview = productURL + linkToReview
		}

		reviewContent := s.Find("div[class^='styles_reviewContent__'] p[class^='typography_body-']").First()

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
