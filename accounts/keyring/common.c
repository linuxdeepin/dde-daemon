#include "common.h"
#include <dlfcn.h>

typedef unsigned char* (*CRYPT_WB)(unsigned char* IN, unsigned char* key);
typedef unsigned char* (*CRYPT_SM4_RET)(unsigned char* IN, unsigned char* key, int mode);
typedef unsigned char* (*CRYPT_SM4_CBC)(unsigned char* IN, unsigned char* key, unsigned char* iv, int mode);
typedef void (*CRYPT_DEBUG)(int state);

void *dalloc(size_t size) {
    void *p = calloc(1, size);
    if (!p) {
        abort();
    }

    return p;
}

char *generate_random_str() {
    static int inited = 0;
    static char *str =
        "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";
    int len = strlen(str);
    char *masterkey = dalloc(MASTER_KEY_LEN + 1);

    if (!inited) {
        inited = 1;
        srand((unsigned)time(NULL));
    }

    for (int i = 0; i < MASTER_KEY_LEN; i++) {
        masterkey[i] = str[rand() % len];
    }
    masterkey[MASTER_KEY_LEN] = '\0';
    return masterkey;
}

char *generate_random_len(unsigned int length) {
    static int inited = 0;
    static char *str = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";
    int len = strlen(str);
    char *masterkey = dalloc(length + 1);

    if (!inited) {
        inited = 1;
        srand((unsigned)time(NULL));
    }

    for (int i = 0; i < length; i++) {
        masterkey[i] = str[rand() % len];
    }
    masterkey[length] = '\0';
    return masterkey;
}

void printState(char *out, unsigned char * in)
{
    printf("%s:\n", out);
    int i;
    int len = strlen(in);
    if (len < MASTER_KEY_LEN) {
        printf(" input length : %d < 16 .\n", len);
        return;
    }
    for(i = 0; i < MASTER_KEY_LEN; i++)
    {
        printf("%.2X", in[i]);
    }
    printf("\n");
}

//flag: false表示key使用16个0（key="",传入后转化位unsigned char*不是16个0）
unsigned char* deepin_wb_encrypt(unsigned char* IN, unsigned char* key, bool flag)
{
    unsigned char *OUT = NULL;
    void *handle = dlopen("/usr/lib/libkeyringcrypto.so", RTLD_NOW);
    if (!handle) {
        printf("failed to dlopen libkeyringcrypto");
        goto over;
    }

    CRYPT_WB func = (CRYPT_WB)dlsym(handle, "deepin_wb_encrypt");
    if (!func) {
        printf("failed to dlsym keyring_crypto");
        goto over;
    }

    char *zero_key = NULL;
    if (!flag) {
        zero_key = dalloc(MASTER_KEY_LEN);
        OUT = func(IN, zero_key);
    } else {
        OUT = func(IN, key);
    }

//    printState("daemon deepin_wb_encrypt IN", IN);
//    printState("daemon deepin_wb_encrypt OUT", OUT);

over:

    if (zero_key)
        free(zero_key);
    return OUT;
}

unsigned char* sm4_crypt(unsigned char* IN, unsigned char* key, int mode)
{
    void *handle = dlopen("/usr/lib/libkeyringcrypto.so", RTLD_NOW);
    if (!handle) {
        printf("failed to dlopen libkeyringcrypto");
        goto over;
    }

    CRYPT_SM4_RET func_sm4 = (CRYPT_SM4_RET)dlsym(handle, "sm4_crypt_ret");
    if (!func_sm4) {
        printf("failed to dlsym sm4_crypt");
        goto over;
    }

    unsigned char *OUT = func_sm4(IN, key, mode);

//    printState("daemon sm4_crypt IN", IN);
//    printf(" sm4_crypt OUT : %s \n", OUT);
//    printState("daemon sm4_crypt OUT2", OUT);

    return OUT;

over:
    return NULL;
}

unsigned char* deepin_crypt_cbc(unsigned char* IN, unsigned char* key, unsigned char* iv, int mode)
{
    void *handle = dlopen("/usr/lib/libkeyringcrypto.so", RTLD_NOW);
    if (!handle) {
        printf("failed to dlopen libkeyringcrypto");
        goto over;
    }

    CRYPT_SM4_CBC func_sm4_cbc = (CRYPT_SM4_CBC)dlsym(handle, "deepin_crypt_cbc");
    if (!func_sm4_cbc) {
        printf("failed to dlsym sm4_crypt");
        goto over;
    }

    unsigned char *OUT = func_sm4_cbc(IN, key, iv, mode);
    return OUT;

over:
    return NULL;
}

void set_debug_flag(int state)
{
    void *handle = dlopen("/usr/lib/libkeyringcrypto.so", RTLD_NOW);
    if (!handle) {
        printf("failed to dlopen libkeyringcrypto");
        goto over;
    }

    CRYPT_DEBUG func_debug = (CRYPT_DEBUG)dlsym(handle, "set_debug_flag");
    if (!func_debug) {
        printf("failed to dlsym set_debug_flag");
        goto over;
    }

    func_debug(state);

over:
    return;
}