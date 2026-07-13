package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)


type Url struct{
	Name string `json:"name"`
}

type Result struct{
	URL string
	StatusCode int
	Err error
	Duration time.Duration
}


func main(){
	var urls []string;

	u1:="https://google.com"
	u2:="https://github.com"
	u3:="https://sdfascfe.com"
	u4:="https://youtube.com"
	u5:="https://claude.ai"
	u6:="https://chatgpt.com"
	urls = append(urls, u1,u2,u3,u4,u5,u6)
	start:=time.Now()
	jobs:=make(chan string, len(urls))
	results:=make(chan Result, len(urls))
	var wg sync.WaitGroup

	for _,url:=range urls{
		jobs<-url
	}
	close(jobs)

	numWorkers:=3
	wg.Add(numWorkers)
	for i:=range numWorkers{
		go worker(i,jobs,results,&wg)
	} 

	go func(){
		wg.Wait()
		close(results)
	}()

	for r:=range results{
		fmt.Printf("%s: %d (%v) \n",r.URL,r.StatusCode,r.Duration)
	}
	timeTaken:=time.Since(start)
	
	fmt.Printf("total time taken: %v\n",timeTaken)

}

func worker(id int,jobs<-chan string, results chan<-Result, wg *sync.WaitGroup){
	defer wg.Done()
	for url:=range jobs{
		fmt.Println("worker",id,"processing",url)
		results<-checkURL(url)
	}
}


func checkURL(url string) Result{
	start:=time.Now()
	resp,err:=http.Get(url)
	end:=time.Since(start)

	var res Result;
	res.URL=url
	res.Err=err
	res.Duration=end
	if err!=nil{
		log.Println("Network/DNS failure:",err)
		return res
	}
	defer resp.Body.Close()
	res.StatusCode=resp.StatusCode;
	if resp.StatusCode!=http.StatusOK{
		log.Printf("Server returned bad status: %d",resp.StatusCode)
		return res
	}
	return res
}