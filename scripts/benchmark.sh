echo "Running benchmark with wrk..."

N_THREADS=12
N_CONNECTIONS=400
DURATION=60s
URL=http://localhost:80

echo "Benchmarking query URL..."
wrk -t$N_THREADS -c$N_CONNECTIONS -d$DURATION -s benchmarks/query_url.lua $URL

echo "Benchmarking create URL..."
wrk -t$N_THREADS -c$N_CONNECTIONS -d$DURATION -s benchmarks/create_url.lua $URL
