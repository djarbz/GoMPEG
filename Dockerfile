FROM jrottenberg/ffmpeg:4-scratch
COPY --chmod=777 gompeg /bin/
CMD         [""]
ENTRYPOINT  ["/bin/gompeg"]
