package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/go-redis/redis"
	"github.com/gofiber/fiber/v2"
)

//Defining global variables to be used
var key = "citymall"
var PORT = ":3000"
var limit int = 1
var R float64 = 1
var cache = redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
}) //Running redis at port 6379

//Function to check if there is a cache hit
func verifyCache(c *fiber.Ctx) error {
	lattitude := c.Params("lat")
	longitude := c.Params("long")

	//Converting longitude string into float
	longFloat, err := strconv.ParseFloat(longitude, 64)
	if err != nil {
		return err
	}

	//Converting lattitude string into float
	latFloat, err := strconv.ParseFloat(lattitude, 64)
	if err != nil {
		return err
	}

	//Checking if there is a pincode availble in R radius
	res, err := cache.GeoRadius(key, longFloat, latFloat, &redis.GeoRadiusQuery{
		Radius:      R,
		Unit:        "km",
		WithGeoHash: false,
		WithCoord:   false,
		WithDist:    true,
		Count:       limit,
		Sort:        "ASC",
	}).Result()

	//If there is no pincode present in availble radius Go to next
	if err != nil || len(res) == 0 {
		return c.Next()
	}

	//Else return cached pincode
	return c.JSON(fiber.Map{"Pincode": res[0].Name})
}

func main() {
	//Initilaizing an app
	app := fiber.New()

	//Get request from specified url with verifyCache as middleware function
	app.Get("/:lat/:long", verifyCache, func(c *fiber.Ctx) error {
		//Getting lattitude and longitude from the url
		lattitude := c.Params("lat")
		longitude := c.Params("long")
		getUrl := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%s&lon=%s", lattitude, longitude)

		//Getting result from openstreetmap api
		res, err := http.Get(getUrl)

		if err != nil {
			return err
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		var location map[string]interface{}
		parseErr := json.Unmarshal([]byte(body), &location)
		if parseErr != nil {
			return parseErr
		}

		if location["error"] != nil {
			return c.SendString("Cannot find pincode corresponding to coordinates")
		}
		address := location["address"].(map[string]interface{})

		longFloat, err := strconv.ParseFloat(longitude, 64)
		if err != nil {
			return err
		}

		latFloat, err := strconv.ParseFloat(lattitude, 64)
		if err != nil {
			return err
		}

		if address["postcode"] == nil {
			return c.SendString("Cannot find pincode corresponding to coordinates")
		}

		//Getting postcode from api response
		var pincode = address["postcode"].(string)

		//Caching pincode using geoadd in redis
		cacheErr := cache.GeoAdd(key, &redis.GeoLocation{
			Longitude: longFloat, Latitude: latFloat, Name: pincode,
		}).Err()
		if cacheErr != nil {
			return cacheErr
		}
		//Returning pincode in json format
		return c.JSON(fiber.Map{"Pincode": pincode})
	})

	//Get for home
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Running server on" + PORT)
	})
	app.Listen(PORT)
}
