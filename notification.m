#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>

#import "notification.h"

static NSMutableDictionary<NSString *, UNNotificationCategory *> *registeredCategories;

void initNotifications(void) {
	registeredCategories = [NSMutableDictionary new];
	UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
	[center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert |
	                                         UNAuthorizationOptionSound |
	                                         UNAuthorizationOptionBadge)
	                      completionHandler:^(BOOL granted, NSError *error) {
		if (error) {
			NSLog(@"Notification authorization error: %@", error);
		}
	}];
}

static NSString *categoryIdentifierForActions(NSString *actionButton,
                                              NSString *closeButton,
                                              NSString *responsePlaceholder) {
	return [NSString stringWithFormat:@"menuet_a:%@_c:%@_r:%@",
	        actionButton ?: @"",
	        closeButton ?: @"",
	        responsePlaceholder ?: @""];
}

static void ensureCategoryRegistered(NSString *actionButton,
                                     NSString *closeButton,
                                     NSString *responsePlaceholder) {
	NSString *categoryId = categoryIdentifierForActions(actionButton,
	                                                    closeButton,
	                                                    responsePlaceholder);
	@synchronized(registeredCategories) {
		if (registeredCategories[categoryId]) {
			return;
		}

		NSMutableArray<UNNotificationAction *> *actions = [NSMutableArray new];

		if (responsePlaceholder.length > 0) {
			UNTextInputNotificationAction *replyAction =
				[UNTextInputNotificationAction actionWithIdentifier:@"menuet.reply"
				                                             title:@"Reply"
				                                           options:UNNotificationActionOptionNone
				                              textInputButtonTitle:@"Send"
				                              textInputPlaceholder:responsePlaceholder];
			[actions addObject:replyAction];
		}

		if (actionButton.length > 0) {
			UNNotificationAction *action =
				[UNNotificationAction actionWithIdentifier:@"menuet.action"
				                                    title:actionButton
				                                  options:UNNotificationActionOptionForeground];
			[actions addObject:action];
		}

		UNNotificationCategoryOptions categoryOptions = UNNotificationCategoryOptionNone;
		if (closeButton.length > 0) {
			categoryOptions |= UNNotificationCategoryOptionCustomDismissAction;
		}

		UNNotificationCategory *category =
			[UNNotificationCategory categoryWithIdentifier:categoryId
			                                       actions:actions
			                             intentIdentifiers:@[]
			                                       options:categoryOptions];
		registeredCategories[categoryId] = category;

		NSSet *categorySet = [NSSet setWithArray:registeredCategories.allValues];
		[[UNUserNotificationCenter currentNotificationCenter]
			setNotificationCategories:categorySet];
	}
}

void showNotification(const char *jsonString) {
	NSDictionary *jsonDict = [NSJSONSerialization
	                          JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
	                                              dataUsingEncoding:NSUTF8StringEncoding]
	                          options:0
	                          error:nil];

	UNMutableNotificationContent *content = [UNMutableNotificationContent new];
	content.title = jsonDict[@"Title"] ?: @"";
	content.subtitle = jsonDict[@"Subtitle"] ?: @"";
	content.body = jsonDict[@"Message"] ?: @"";
	content.sound = [UNNotificationSound defaultSound];

	NSString *actionButton = jsonDict[@"ActionButton"];
	NSString *closeButton = jsonDict[@"CloseButton"];
	NSString *responsePlaceholder = jsonDict[@"ResponsePlaceholder"];

	BOOL needsCategory = (actionButton.length > 0 ||
	                       closeButton.length > 0 ||
	                       responsePlaceholder.length > 0);
	if (needsCategory) {
		ensureCategoryRegistered(actionButton, closeButton, responsePlaceholder);
		content.categoryIdentifier = categoryIdentifierForActions(actionButton,
		                                                          closeButton,
		                                                          responsePlaceholder);
	}

	NSString *identifier = jsonDict[@"Identifier"];
	if (identifier.length == 0) {
		identifier = [[NSUUID UUID] UUIDString];
	}

	BOOL removeFromNotificationCenter = [jsonDict[@"RemoveFromNotificationCenter"] boolValue];

	UNNotificationRequest *request =
		[UNNotificationRequest requestWithIdentifier:identifier
		                                     content:content
		                                     trigger:nil];
	dispatch_async(dispatch_get_main_queue(), ^{
		UNUserNotificationCenter *center =
			[UNUserNotificationCenter currentNotificationCenter];
		[center addNotificationRequest:request
		         withCompletionHandler:^(NSError *error) {
			if (error) {
				NSLog(@"Notification error: %@", error);
			}
			if (removeFromNotificationCenter) {
				[center removeDeliveredNotificationsWithIdentifiers:@[identifier]];
			}
		}];
	});
}
