package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

type Histo struct {
	Name string
	H    [512]float64
}

type intersection struct {
	Name       string
	similarity float64
}

// adapted from: first example at pkg.go.dev/image
func computeHistogram(imagePath string, depth int) (Histo, error) {
	// Open the JPEG file
	file, err := os.Open(imagePath)
	var values [512]float64

	if err != nil {
		return Histo{"", values}, err
	}
	defer file.Close()

	// Decode the JPEG image
	img, _, err := image.Decode(file)
	if err != nil {
		return Histo{"", values}, err
	}

	// Get the dimensions of the image
	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y
	sum := 0.0

	// Display RGB values for the first 5x5 pixels
	// remove y < 5 and x < 5  to scan the entire image
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {

			// Convert the pixel to RGBA
			red, green, blue, _ := img.At(x, y).RGBA()
			// A color's RGBA method returns values in the range [0, 65535].
			// Shifting by 8 reduces this to the range [0, 255].
			red >>= 16 - depth
			blue >>= 16 - depth
			green >>= 16 - depth

			// calculate position in histogram array
			pos := ((red << (2 * depth)) + (green << depth) + blue)
			values[pos]++
			sum++

		}
	}

	for i := 0; i < len(values); i++ {
		values[i] = values[i] / sum
	}

	h := Histo{imagePath, values}
	return h, nil
}

func computeHistograms(imagePath []string, depth int, hChan chan<- Histo) {
	for _, path := range imagePath {
		h, err := computeHistogram(path, depth)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
		hChan <- h
	}
	close(hChan)
}

func compareHistograms(query Histo, data Histo) float64 {

	result := 0.0

	for i := 0; i < len(data.H); i++ {
		result = result + math.Min(query.H[i], data.H[i])
	}

	return result
}

func KNN(result []intersection, data intersection) {
	//Sort the results by similarity
	//Print the top 5 results

	if len(result) < 5 {
		result = append(result, data)
	} else {
		// Find the position where the new data should be inserted
		pos := -1
		for i := 0; i < 5; i++ {
			if data.similarity > result[i].similarity {
				pos = i
				break
			}
		}

		// If the new data's similarity is greater than at least one element in the result slice
		if pos != -1 {
			// Shift the elements from pos to the end one position to the right
			for i := 4; i > pos; i-- {
				result[i] = result[i-1]
			}
			// Insert the new data at pos
			result[pos] = data
		}

	}
}

func main() {
	// read the image name from command line
	args := os.Args

	//Store the runtime of the program
	startTime := time.Now()

	K := 16

	//In main channel compute query histogram
	query, err := computeHistogram(args[1], 3)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	// Create a channel to store the histogram of all images
	dChan := make([]chan Histo, K)

	//Get image file names
	// read the directory name from command line

	files, err := ioutil.ReadDir(args[2])
	if err != nil {
		log.Fatal(err)
	}

	// Create an array to store filenames
	var filenames []string

	// get the list of jpg files
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".jpg") {
			input := args[2] + "/" + file.Name()
			filenames = append(filenames, input)
		}
	}

	//Make an 2D array of K by filenames/k length
	a := make([][]string, 0)

	// Calculate the size of each sub-slice
	size := len(filenames) / K
	remainder := len(filenames) % K

	// Create the sub-slices
	for i := 0; i < K; i++ {
		start := i * size
		end := start + size
		if i == K-1 {
			// Add the remainder to the last slice
			end += remainder
		}
		a = append(a, filenames[start:end])
	}

	// Create a channel for each sub-slice
	for i := 0; i < K; i++ {
		dChan[i] = make(chan Histo)
		go computeHistograms(a[i], 3, dChan[i])
	}

	// Create a WaitGroup and a results slice
	var wg sync.WaitGroup
	results := make([]intersection, 5)

	// Launch a goroutine for each channel
	ran := 0
	for i := 0; i < K; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for histo := range dChan[i] {
				intersect := intersection{histo.Name, compareHistograms(query, histo)}

				// Add the intersection to the results slice
				KNN(results, intersect)

				ran++
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	endTime := time.Now()

	execTime := endTime.Sub(startTime)

	//////////////PRINTS FOR TESTING/////////////////////

	fmt.Println("The top 5  most similar images to ", args[1], " are: ")
	for i := 0; i < len(results); i++ {
		fmt.Println((i + 1), ": ", results[i].Name, "Similarity: ", results[i].similarity)
	}
	fmt.Println("Execution time: ", execTime)

}
