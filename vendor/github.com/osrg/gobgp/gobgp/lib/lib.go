// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
// typedef struct {
//     char *value;
//     int len;
// } buf;
//
// typedef struct path_t {
//     buf   nlri;
//     buf** path_attributes;
//     int   path_attributes_len;
//     int   path_attributes_cap;
// } path;
//
// path* new_path() {
//	path* p;
//	int cap = 32;
//	p = (path*)malloc(sizeof(path));
//	memset(p, 0, sizeof(path));
//	p->nlri.len = 0;
//	p->path_attributes_len = 0;
//	p->path_attributes_cap = cap;
//	p->path_attributes = (buf**)malloc(sizeof(buf)*cap);
//	return p;
// }
//
// void free_path(path* p) {
//	int i;
//	if (p->nlri.value != NULL) {
//	    free(p->nlri.value);
//	}
//	for (i = 0; i < p->path_attributes_len; i++) {
//	    buf* b;
//	    b = p->path_attributes[i];
//	    free(b->value);
//	    free(b);
//	}
//	free(p->path_attributes);
//	free(p);
// }
//
// int append_path_attribute(path* p, int len, char* value) {
//	buf* b;
//	if (p->path_attributes_len >= p->path_attributes_cap) {
//	    return -1;
//	}
//	b = (buf*)malloc(sizeof(buf));
//	b->value = value;
//	b->len = len;
//	p->path_attributes[p->path_attributes_len] = b;
//	p->path_attributes_len++;
//	return 0;
// }
// buf* get_path_attribute(path* p, int idx) {
//	if (idx < 0 || idx >= p->path_attributes_len) {
//	    return NULL;
//	}
//	return p->path_attributes[idx];
// }
import "C"
