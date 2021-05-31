package main

import (
	"ServiceB/tracer"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"log"
	"net/http"
	"os"
)

const (
	tracingConfigKey = "tracing"
	applicationName  = "ServiceB"
)

func reqInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		log.Println(fmt.Sprintf("Incoming request headers.... %v",r.Header))

		next.ServeHTTP(w, r)
	})
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	traceID, spanID, _ := tracer.ExtractTraceInfo(r.Context())
	log.Println(fmt.Sprintf("Trace ID for this request in %s is: %s and Span Id is: %s", applicationName, traceID, spanID))

	fmt.Println("Request Received...")
}

func getViper() *viper.Viper {
	viper.SetConfigName("config")

	// Set the path to look for the configurations file
	viper.AddConfigPath(".")

	// Enable VIPER to read Environment Variables
	viper.AutomaticEnv()

	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	return viper.GetViper()

}

func initTracing(v *viper.Viper, appName string) (tracer.Tracing, error) {
	var tracing = tracer.Tracing{
		Propagator:     propagation.TraceContext{},
		TracerProvider: trace.NewNoopTracerProvider(),
	}
	var traceConfig tracer.Config
	err := v.UnmarshalKey(tracingConfigKey, &traceConfig)
	if err != nil {
		return tracer.Tracing{}, err
	}
	traceConfig.ApplicationName = appName
	tracerProvider, err := tracer.ConfigureTracerProvider(traceConfig)
	if err != nil {
		return tracer.Tracing{}, err
	}

	tracing.TracerProvider = tracerProvider
	return tracing, nil
}

func main() {

	v := getViper()

	tracing, err := initTracing(v, applicationName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to build tracing component: %v \n", err)
		return
	}

	// Auto instrumentation options of mux router.
	otelMuxOptions := []otelmux.Option{
		otelmux.WithPropagators(tracing.Propagator),
		otelmux.WithTracerProvider(tracing.TracerProvider),
	}

	r := mux.NewRouter()

	r.Use(otelmux.Middleware("primary", otelMuxOptions...), reqInterceptor, tracer.EchoFirstTraceNodeInfo(tracing.Propagator))

	r.HandleFunc("/", requestHandler)
	err1 := http.ListenAndServe(":"+v.GetString("port"), r)
	log.Fatal(err1)

}
