#import <Foundation/Foundation.h>
#import <Security/Security.h>
#include <string.h>

static NSMutableDictionary *webhookKeyQuery(const char *account) {
    return [NSMutableDictionary dictionaryWithDictionary:@{
        (id)kSecClass: (id)kSecClassGenericPassword,
        (id)kSecAttrService: @"com.wails.hostlight.webhooks",
        (id)kSecAttrAccount: [NSString stringWithUTF8String:account],
    }];
}

OSStatus hostlightReadWebhookKey(const char *account, unsigned char *key, int size) {
    @autoreleasepool {
        NSMutableDictionary *query = webhookKeyQuery(account);
        query[(id)kSecReturnData] = @YES;
        query[(id)kSecMatchLimit] = (id)kSecMatchLimitOne;
        CFTypeRef result = NULL;
        OSStatus status = SecItemCopyMatching((CFDictionaryRef)query, &result);
        if (status == errSecSuccess) {
            NSData *data = (NSData *)result;
            if (data.length == (NSUInteger)size) {
                memcpy(key, data.bytes, (size_t)size);
            } else {
                status = errSecDecode;
            }
            CFRelease(result);
        }
        return status;
    }
}

OSStatus hostlightAddWebhookKey(const char *account, const unsigned char *key, int size) {
    @autoreleasepool {
        NSMutableDictionary *query = webhookKeyQuery(account);
        query[(id)kSecAttrLabel] = @"Hostlight webhook encryption key";
        query[(id)kSecValueData] = [NSData dataWithBytes:key length:(NSUInteger)size];
        return SecItemAdd((CFDictionaryRef)query, NULL);
    }
}
