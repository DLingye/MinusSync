CC      ?= gcc
CFLAGS  ?= -Wall -Wextra -O2
PREFIX  ?= /usr/local
BINDIR  ?= $(PREFIX)/bin

UNAME_S := $(shell uname -s 2>/dev/null || echo Windows)

SRCS = src/main.c src/sha256.c src/util.c src/object.c src/index.c \
       src/refs.c src/diff.c src/net.c src/config.c src/commit.c \
       src/init.c src/status.c src/repo.c src/log.c src/branch.c \
       src/checkout.c src/push.c src/update.c src/clone.c src/serve.c \
       src/remote.c src/mirror.c

OBJS = $(SRCS:.c=.o)
TARGET = msync

ifeq ($(UNAME_S),Windows)
    TARGET := msync.exe
    LIBS   := -lws2_32
else ifeq ($(UNAME_S),MINGW32_NT-6.2)
    TARGET := msync.exe
    LIBS   := -lws2_32
else ifeq ($(UNAME_S),MINGW64_NT-10.0)
    TARGET := msync.exe
    LIBS   := -lws2_32
else ifeq ($(findstring MINGW,$(UNAME_S)),MINGW)
    TARGET := msync.exe
    LIBS   := -lws2_32
else
    LIBS   :=
endif

.PHONY: all clean install uninstall

all: $(TARGET)

$(TARGET): $(OBJS)
	$(CC) $(CFLAGS) -o $@ $^ $(LIBS)

src/%.o: src/%.c src/msync.h src/sha256.h
	$(CC) $(CFLAGS) -c -o $@ $<

clean:
	rm -f $(OBJS) $(TARGET) msync.exe

install: $(TARGET)
	install -d $(BINDIR)
	install -m 755 $(TARGET) $(BINDIR)/$(TARGET)

uninstall:
	rm -f $(BINDIR)/$(TARGET)
