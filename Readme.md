# TrustPilot Product/Company reviews scraper

This project is a web scraper for TrustPilot product/company reviews. It uses the BeautifulSoup library to parse the HTML content of the TrustPilot website and extract the reviews.

## Getting Started

To use this project, you will need to have Go installed on your machine. You will also need to install the following Go packages:

* github.com/PuerkitoBio/goquery

To run the scraper, simply run the following command by providing the product name as an argument:

```sh
    go run main.go -product="product_name"
```

As a result it will create a json file with the reviews scraped which will be located in the same directory as the main.go file.

## License
This project is licensed under the MIT License.

