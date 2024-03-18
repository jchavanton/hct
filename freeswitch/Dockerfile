FROM debian:bookworm

RUN apt-get update \
    && apt-get dist-upgrade -y \
    && apt install -y \
        autoconf \
        automake \
        build-essential \
        cmake \
        ffmpeg \
        git \
        libasound2-dev \
        libavformat-dev \
        libcurl4-openssl-dev \
        libdb-dev \
        libedit-dev \
        libexpat1-dev \
        libgdbm-dev \
        libgnutls28-dev \
        libjpeg-dev \
        libldns-dev \
        liblua5.1-0-dev  \
        liblua5.2-dev \
        libncurses5 \
        libncurses5-dev \
        libogg-dev \
        libopus-dev \
        libpcre3-dev \
        libperl-dev \
        libpq-dev \
        libsndfile1-dev \
        libspeex-dev \
        libspeexdsp-dev \
        libsqlite3-dev \
        libssl-dev \
        libswscale-dev \
        libtiff5-dev \
        libtool \
        libtool-bin \
        libvorbis-dev \
        libx11-dev \
        lua5.1  \
        luarocks \
        make \
        net-tools \
        python-dev-is-python3 \
        subversion \
        unixodbc-dev \
        unzip \
        uuid-dev \
        vim \
        wget \
        yasm \
        zlib1g-dev \
        lua-json \
    && apt-get clean \
    && rm -r /var/lib/apt/lists/*
RUN luarocks install luasocket

ENV FS_VERSION="1.10.9"

RUN echo "building signalwire-c ..." \
   && cd /usr/src \
   && git clone https://github.com/signalwire/libks.git \
   && cd libks \
   && cmake . \
   && make \
   && make install \
   && cd /usr/src \
   && git clone https://github.com/signalwire/signalwire-c.git \
   && cd signalwire-c \
   && cmake . \
   && make \
   && make install

RUN echo "building sofia-sip" \
   && cd /usr/src \
   && git clone https://github.com/freeswitch/sofia-sip \
   && cd sofia-sip && ./bootstrap.sh && ./configure && make install

RUN echo "building spandsp" \
   && cd /usr/src \
   && git clone https://github.com/freeswitch/spandsp \
   && cd spandsp && git checkout 728b60ab && ./bootstrap.sh && ./configure && make install && ldconfig -v

# Downgrade to OpenSSL V1
RUN mkdir -p /opt/openssl \
   && wget https://www.openssl.org/source/openssl-1.1.1u.tar.gz --no-check-certificate -P /opt/openssl/ \
   && tar -xzvf /opt/openssl/openssl-1.1.1u.tar.gz -C /opt/openssl/ \
   && cd /opt/openssl/openssl-1.1.1u \
   && ./config \
   && make install && ldconfig

ENV GIT_COMMIT="4137bca"

RUN echo "download freeswitch src ..." \
   && cd /usr/src && rm -rf /usr/src/freeswitch \
   && git clone https://github.com/jchavanton/freeswitch \
   && cd freeswitch && git checkout ${GIT_COMMIT}

RUN echo "configure building freeswitch ..." \
   && cd /usr/src/freeswitch \
   && ./bootstrap.sh && ./configure  --disable-core-pgsql-pkgconfig

RUN echo "make building freeswitch ..." \
   && cd /usr/src/freeswitch \
   && make -Wno-address \
   && make install

RUN echo "build complete, uninstalling packages ..." \
   && apt-get remove -y unzip && apt-get purge -y unzip

COPY vars.xml /usr/local/freeswitch/conf/
COPY 00_inbound_did.xml /usr/local/freeswitch/conf/dialplan/public/
COPY hangup_metrics.lua /hangup_metrics.lua
COPY opus.conf.xml /usr/local/freeswitch/conf/autoload_configs/
COPY switch.conf.xml /usr/local/freeswitch/conf/autoload_configs/
COPY external.xml /usr/local/freeswitch/conf/sip_profiles/
COPY internal.xml /usr/local/freeswitch/conf/sip_profiles/
COPY Dockerfile /
COPY entry.sh /
ENTRYPOINT ["/entry.sh"]
