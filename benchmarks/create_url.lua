-- Seed random number generator per thread
math.randomseed(os.time())

request = function()
  local unique_suffix = os.time() .. "-" .. math.random(1000, 9999)
  local long_url = "http://very.long.url.example.com/path/to/resource?id=" .. unique_suffix
  
  wrk.method = "POST"
  wrk.path = "/create"
  wrk.headers["Content-Type"] = "application/json"
  wrk.body = string.format('{"long_url": "%s"}', long_url)
  
  return wrk.format() -- This is important!
end

response = function(status, headers, body)
  if status ~= 200 and status ~= 201 then
    print("POST /create failed with status: " .. status)
  end
end