FROM public.ecr.aws/lambda/python:3.8

# networking tools
RUN yum install -y iproute lsof iputils bind-utils
# process management
RUN yum install -y htop psmisc procps-ng lsof strace
# misc
RUN yum install -y vim pstree ncdu

# add /usr/sbin since some packages install binaries here and the image doesn't have it in path by default
ENV PATH=/var/lang/bin:/usr/local/bin:/usr/bin/:/bin:/opt/bin:/usr/sbin

COPY app.py ${LAMBDA_TASK_ROOT}

CMD ["app.handler"]
