FROM justb4/jmeter:latest
COPY tests/ /tests/
COPY run-load-test.sh /run-load-test.sh
RUN chmod +x /run-load-test.sh
WORKDIR /
CMD ["/run-load-test.sh"]


