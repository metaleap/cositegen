killall cositegen
killall chromium
GO111MODULE=off go install
cd colorizer && npm run dev
