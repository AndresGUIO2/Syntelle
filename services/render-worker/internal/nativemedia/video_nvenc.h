#ifndef SYNTELLE_NVENC_H
#define SYNTELLE_NVENC_H
#include <stdint.h>
typedef struct syntelle_nvenc syntelle_nvenc;
syntelle_nvenc *syntelle_nvenc_open(int width, int height, int fps, int bitrate, int gpu, int *status);
int syntelle_nvenc_frame(syntelle_nvenc *encoder, uint8_t **y, uint8_t **u, uint8_t **v, int *ys, int *cs);
int syntelle_nvenc_encode(syntelle_nvenc *encoder, int key, uint8_t **data, int *size);
int syntelle_nvenc_surface(syntelle_nvenc*,uint64_t *context,uint64_t *planes,int *strides);
int syntelle_nvenc_encode_device(syntelle_nvenc*,int key,uint8_t **data,int *size);
int syntelle_nvenc_download(syntelle_nvenc*);
void syntelle_nvenc_close(syntelle_nvenc *encoder);
#endif
