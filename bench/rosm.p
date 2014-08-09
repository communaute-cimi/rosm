# ab -c 10000 -n 10500 -g rosm-go.dat http://127.0.0.1:8080/0/0/0.png
# ab -c 10000 -n 10500 -g rosm-py.dat http://127.0.0.1:8000/0/0/0.png

#output as png image
set terminal png

set output "rosm.png"

# graph title
set title "ab -n 10000 -c 10500"

# nicer aspect ratio for image size
set size 1,0.7

# y-axis grid
set grid y

# x-axis label
set xlabel "request"

# y-axis label
set ylabel "response time (ms)"

plot "rosm-go.dat" using 9 smooth sbezier with lines title "go", \
 "rosm-py.dat" using 9 smooth sbezier with lines title "django wsgi"
